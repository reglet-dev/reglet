package hostfuncs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv" // Import strconv for proper type conversion
	"time"

	"github.com/tetratelabs/wazero/api"
)

type logContextKey string

const requestIDKey logContextKey = "request_id"

// LogMessageWire is the JSON wire format for a log message from Guest to Host.
type LogMessageWire struct {
	Context   ContextWireFormat `json:"context"` // Context for correlation etc.
	Level     string            `json:"level"`
	Message   string            `json:"message"`
	Timestamp time.Time         `json:"timestamp"`
	Attrs     []LogAttrWire     `json:"attrs,omitempty"`
}

// LogAttrWire represents a single slog attribute.
type LogAttrWire struct {
	Key   string `json:"key"`
	Type  string `json:"type"`  // "string", "int64", "bool", "float64", "time", "error", "any"
	Value string `json:"value"` // String representation of the value
}

// LogMessage implements the `log_message` host function.
// It receives a packed uint64 (ptr+len) pointing to a JSON-encoded LogMessageWire.
// It does not return any value.
func LogMessage(ctx context.Context, mod api.Module, stack []uint64) {
	// Stack contains a single uint64 which is packed ptr+len of the log message.
	messagePacked := stack[0]
	ptr, length := unpackPtrLen(messagePacked)

	messageBytes, ok := mod.Memory().Read(ptr, length)
	if !ok {
		slog.Error("hostfuncs: failed to read log message from Guest memory")
		return // Cannot return error to Guest
	}

	var logMsg LogMessageWire
	if err := json.Unmarshal(messageBytes, &logMsg); err != nil {
		slog.Error("hostfuncs: failed to unmarshal log message", "error", err)
		return
	}

	// Create a log context with correlation ID if available
	logCtx, _ := createContextFromWire(ctx, logMsg.Context)
	if logMsg.Context.RequestID != "" {
		logCtx = context.WithValue(logCtx, requestIDKey, logMsg.Context.RequestID)
	}

	// Determine slog level
	level := slog.LevelInfo
	if err := level.UnmarshalText([]byte(logMsg.Level)); err != nil {
		slog.Warn("hostfuncs: unknown log level from plugin", "level", logMsg.Level, "message", logMsg.Message)
		level = slog.LevelInfo
	}

	// Convert wire attributes to slog.Attr
	attrs := make([]slog.Attr, 0, len(logMsg.Attrs))
	for _, attr := range logMsg.Attrs {
		switch attr.Type {
		case "string":
			attrs = append(attrs, slog.String(attr.Key, attr.Value))
		case "int64":
			if v, err := strconv.ParseInt(attr.Value, 10, 64); err == nil {
				attrs = append(attrs, slog.Int64(attr.Key, v))
			} else {
				attrs = append(attrs, slog.String(attr.Key, attr.Value)) // Fallback to string
			}
		case "bool":
			if v, err := strconv.ParseBool(attr.Value); err == nil {
				attrs = append(attrs, slog.Bool(attr.Key, v))
			} else {
				attrs = append(attrs, slog.String(attr.Key, attr.Value)) // Fallback to string
			}
		case "float64":
			if v, err := strconv.ParseFloat(attr.Value, 64); err == nil {
				attrs = append(attrs, slog.Float64(attr.Key, v))
			} else {
				attrs = append(attrs, slog.String(attr.Key, attr.Value)) // Fallback to string
			}
		case "time":
			if v, err := time.Parse(time.RFC3339Nano, attr.Value); err == nil {
				attrs = append(attrs, slog.Time(attr.Key, v))
			} else {
				attrs = append(attrs, slog.String(attr.Key, attr.Value)) // Fallback to string
			}
		case "error":
			attrs = append(attrs, slog.Any(attr.Key, fmt.Errorf("%s", attr.Value))) // Re-wrap as an error, ensuring format string.
		default:
			attrs = append(attrs, slog.Any(attr.Key, attr.Value))
		}
	}

	// Log with host's slog instance
	slog.LogAttrs(logCtx, level, logMsg.Message, attrs...)
}
