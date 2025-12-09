package context

import (
	stdcontext "context"
	"sync"
	"time"

	"github.com/whiskeyjimbo/reglet/wireformat"
)

// contextStore holds the current context for the plugin execution.
// Since WASM is single-threaded, we can use a simple global variable.
// The host sets this when calling into the plugin via SetCurrentContext.
var contextStore = struct {
	sync.RWMutex
	ctx stdcontext.Context
}{
	ctx: stdcontext.Background(),
}

// SetCurrentContext sets the current execution context.
// This is called by the plugin infrastructure when the host invokes
// an exported function (describe, schema, observe).
func SetCurrentContext(ctx stdcontext.Context) {
	contextStore.Lock()
	defer contextStore.Unlock()
	contextStore.ctx = ctx
}

// GetCurrentContext returns the current execution context.
// This is used by SDK functions (exec, net, log) to get the context
// for host function calls.
func GetCurrentContext() stdcontext.Context {
	contextStore.RLock()
	defer contextStore.RUnlock()
	if contextStore.ctx == nil {
		return stdcontext.Background()
	}
	return contextStore.ctx
}

// ContextToWire converts a stdcontext.Context to ContextWireFormat for
// sending to the host via host functions.
func ContextToWire(ctx stdcontext.Context) wireformat.ContextWireFormat {
	wire := wireformat.ContextWireFormat{}

	// Extract deadline
	if deadline, ok := ctx.Deadline(); ok {
		wire.Deadline = &deadline
		// Calculate timeout in milliseconds
		timeout := time.Until(deadline)
		if timeout > 0 {
			wire.TimeoutMs = timeout.Milliseconds()
		}
	}

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		wire.Cancelled = true
	default:
		wire.Cancelled = false
	}

	// Extract request ID from context if available
	if requestID := ctx.Value("request_id"); requestID != nil {
		if id, ok := requestID.(string); ok {
			wire.RequestID = id
		}
	}

	return wire
}

// WireToContext converts a ContextWireFormat to a stdcontext.Context.
// This is used when the host provides context information to the plugin.
func WireToContext(parent stdcontext.Context, wire wireformat.ContextWireFormat) (stdcontext.Context, stdcontext.CancelFunc) {
	if parent == nil {
		parent = stdcontext.Background()
	}

	ctx := parent

	// Apply deadline if present
	var cancel stdcontext.CancelFunc
	if wire.Deadline != nil {
		ctx, cancel = stdcontext.WithDeadline(ctx, *wire.Deadline)
	} else if wire.TimeoutMs > 0 {
		ctx, cancel = stdcontext.WithTimeout(ctx, time.Duration(wire.TimeoutMs)*time.Millisecond)
	} else {
		ctx, cancel = stdcontext.WithCancel(ctx)
	}

	// Add request ID to context if present
	if wire.RequestID != "" {
		ctx = stdcontext.WithValue(ctx, "request_id", wire.RequestID)
	}

	// If context is already cancelled, cancel immediately
	if wire.Cancelled {
		cancel()
	}

	return ctx, cancel
}
