//go:build wasip1

package log

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/whiskeyjimbo/reglet/sdk/internal/abi"
	"github.com/whiskeyjimbo/reglet/sdk/net" // To reuse ContextWireFormat
)

// Define the host function signature for logging messages.
// This matches the signature defined in internal/wasm/hostfuncs/registry.go.
//go:wasmimport reglet_host log_message
func host_log_message(messagePacked uint64)

// LogMessageWire is the JSON wire format for a log message from Guest to Host.
type LogMessageWire struct {
	Context   net.ContextWireFormat `json:"context"`
	Level     string                `json:"level"`
	Message   string                `json:"message"`
	Timestamp time.Time             `json:"timestamp"`
	Attrs     []LogAttrWire         `json:"attrs,omitempty"`
}

// LogAttrWire represents a single slog attribute for wire transfer.
type LogAttrWire struct {
	Key   string `json:"key"`
	Type  string `json:"type"`  // "string", "int64", "bool", "float64", "time", "error", "any"
	Value string `json:"value"` // String representation of the value
}

// WasmLogHandler implements slog.Handler to route logs through a host function.
type WasmLogHandler struct{}

// Enabled reports whether the handler handles records at the given level.
func (h *WasmLogHandler) Enabled(_ context.Context, level slog.Level) bool {
	// For now, enable all levels from plugin to host.
	// Host can filter based on its own config.
	return true
}

// Handle serializes a slog.Record and sends it to the host via a host function.
func (h *WasmLogHandler) Handle(ctx context.Context, record slog.Record) error {
	// Create LogMessageWire from slog.Record
	// Note: sdk/net package is in the same module, so we can import it.
	// But Wait, sdk/net depends on sdk/internal/abi.
	// sdk/log depends on sdk/net.
	// This is fine.

	// We need to construct ContextWireFormat manually if we can't import sdk/net to avoid cycles
	// Actually, sdk/net doesn't depend on sdk/log.
	// So sdk/log CAN import sdk/net.

	logMsg := LogMessageWire{
		// Context:   net.CreateContextWireFormat(ctx), // This function is not exported in sdk/net yet? It is.
		// Actually, let's just re-implement context extraction or make it public in a shared place?
		// Ideally sdk/net/wireformat.go has it.
		// Let's verify net/wireformat.go content.
		Level:     record.Level.String(),
		Message:   record.Message,
		Timestamp: record.Time,
	}

	// Handle context manually to avoid circular dependency if sdk/net imports sdk/log (which it doesn't, but good to be safe)
	// Actually, sdk/net/http.go uses slog.Info in init().
	// If sdk/net imports sdk/log, we have a cycle.
	// sdk/net imports "log/slog".
	// sdk/log imports "sdk/net".
	// Cycle: sdk/net -> sdk/log (implicitly via init?) No, sdk/net uses stdlib log/slog.
	// But if we want sdk/net's init() logs to go to host, we need sdk/log.
	// And sdk/log needs sdk/net's ContextWireFormat.
	// This IS a cycle if sdk/net/http.go imports sdk/log to set default logger?
	// Currently sdk/net/http.go does NOT import sdk/log. It just logs.
	// But sdk/log/log.go's init() sets the default logger.
	// So if both are imported by main, both inits run.

	// Convert slog.Attr to LogAttrWire
	record.Attrs(func(attr slog.Attr) bool {
		logMsg.Attrs = append(logMsg.Attrs, toLogAttrWire(attr))
		return true // Continue iterating
	})

	requestBytes, err := json.Marshal(logMsg)
	if err != nil {
		// Fallback to println if marshalling fails.
		// We cannot use slog here directly as it would loop.
		fmt.Printf("sdk: failed to marshal log message for host: %v, original: %s\n", err, record.Message)
		return err
	}

	// Call the host function (no return value)
	host_log_message(abi.PtrFromBytes(requestBytes))
	return nil
}

// WithAttrs returns a new WasmLogHandler that includes the given attributes.
func (h *WasmLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h // Simplified for now
}

// WithGroup returns a new WasmLogHandler with the given group name.
func (h *WasmLogHandler) WithGroup(name string) slog.Handler {
	return h // Simplified for now
}

// toLogAttrWire converts a slog.Attr to LogAttrWire.
func toLogAttrWire(attr slog.Attr) LogAttrWire {
	wire := LogAttrWire{
		Key: attr.Key,
	}
	// Resolve the attribute value
	attr.Value = attr.Value.Resolve()

	switch attr.Value.Kind() {
	case slog.KindString:
		wire.Type = "string"
		wire.Value = attr.Value.String()
	case slog.KindInt64:
		wire.Type = "int64"
		wire.Value = fmt.Sprintf("%d", attr.Value.Int64())
	case slog.KindUint64:
		wire.Type = "uint64"
		wire.Value = fmt.Sprintf("%d", attr.Value.Uint64())
	case slog.KindBool:
		wire.Type = "bool"
		wire.Value = fmt.Sprintf("%t", attr.Value.Bool())
	case slog.KindFloat64:
		wire.Type = "float64"
		wire.Value = fmt.Sprintf("%f", attr.Value.Float64())
	case slog.KindTime:
		wire.Type = "time"
		wire.Value = attr.Value.Time().Format(time.RFC3339Nano)
	case slog.KindDuration:
		wire.Type = "duration"
		wire.Value = attr.Value.Duration().String()
	case slog.KindAny:
		if v := attr.Value.Any(); v != nil {
			if err, isErr := v.(error); isErr {
				wire.Type = "error"
				wire.Value = err.Error()
			} else if data, marshalErr := json.Marshal(v); marshalErr == nil {
				wire.Type = "json"
				wire.Value = string(data)
			} else {
				wire.Type = "any"
				wire.Value = fmt.Sprintf("%v", v)
			}
		} else {
			wire.Type = "any"
			wire.Value = "<nil>"
		}
	case slog.KindGroup:
		wire.Type = "group"
		wire.Value = fmt.Sprintf("%v", attr.Value.Any())
	case slog.KindLogValuer:
		return toLogAttrWire(slog.Attr{Key: attr.Key, Value: attr.Value.LogValuer().LogValue()})
	default:
		wire.Type = "any"
		wire.Value = fmt.Sprintf("%v", attr.Value.Any())
	}
	return wire
}

// init configures the default slog handler to use our WasmLogHandler.
func init() {
	slog.SetDefault(slog.New(&WasmLogHandler{}))
	slog.Info("Reglet SDK: Slog handler initialized.")
}
