package hostfuncs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
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
	logMsg, ok := readLogMessage(ctx, mod, stack[0])
	if !ok {
		return
	}

	logCtx := buildLogContext(ctx, logMsg)
	level := parseLogLevel(logMsg.Level)
	attrs := convertLogAttrs(logMsg.Attrs)

	slog.LogAttrs(logCtx, level, logMsg.Message, attrs...)
}

// readLogMessage reads and unmarshals the log message from guest memory.
func readLogMessage(ctx context.Context, mod api.Module, messagePacked uint64) (*LogMessageWire, bool) {
	ptr, length := unpackPtrLen(messagePacked)

	messageBytes, ok := mod.Memory().Read(ptr, length)
	if !ok {
		slog.ErrorContext(ctx, "hostfuncs: failed to read log message from Guest memory")
		return nil, false
	}

	var logMsg LogMessageWire
	if err := json.Unmarshal(messageBytes, &logMsg); err != nil {
		slog.ErrorContext(ctx, "hostfuncs: failed to unmarshal log message", "error", err)
		return nil, false
	}

	return &logMsg, true
}

// buildLogContext creates a log context with correlation ID if available.
func buildLogContext(ctx context.Context, logMsg *LogMessageWire) context.Context {
	logCtx, _ := createContextFromWire(ctx, logMsg.Context)
	if logMsg.Context.RequestID != "" {
		logCtx = context.WithValue(logCtx, requestIDKey, logMsg.Context.RequestID)
	}
	return logCtx
}

// parseLogLevel converts a string level to slog.Level.
func parseLogLevel(levelStr string) slog.Level {
	level := slog.LevelInfo
	if err := level.UnmarshalText([]byte(levelStr)); err != nil {
		slog.Warn("hostfuncs: unknown log level from plugin", "level", levelStr)
	}
	return level
}

// convertLogAttrs converts wire attributes to slog.Attr slice.
func convertLogAttrs(wireAttrs []LogAttrWire) []slog.Attr {
	attrs := make([]slog.Attr, 0, len(wireAttrs))
	for _, attr := range wireAttrs {
		attrs = append(attrs, convertSingleAttr(attr))
	}
	return attrs
}

// convertSingleAttr converts a single wire attribute to slog.Attr.
func convertSingleAttr(attr LogAttrWire) slog.Attr {
	switch attr.Type {
	case "string":
		return slog.String(attr.Key, attr.Value)
	case "int64":
		if v, err := strconv.ParseInt(attr.Value, 10, 64); err == nil {
			return slog.Int64(attr.Key, v)
		}
	case "bool":
		if v, err := strconv.ParseBool(attr.Value); err == nil {
			return slog.Bool(attr.Key, v)
		}
	case "float64":
		if v, err := strconv.ParseFloat(attr.Value, 64); err == nil {
			return slog.Float64(attr.Key, v)
		}
	case "time":
		if v, err := time.Parse(time.RFC3339Nano, attr.Value); err == nil {
			return slog.Time(attr.Key, v)
		}
	case "error":
		return slog.Any(attr.Key, fmt.Errorf("%s", attr.Value))
	}
	// Default: return as Any (fallback for unknown types or parse failures)
	return slog.Any(attr.Key, attr.Value)
}
