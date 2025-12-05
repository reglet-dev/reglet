package hostfuncs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/tetratelabs/wazero/api"
	"github.com/whiskeyjimbo/reglet/wireformat"
)

// Re-export wire format types from shared wireformat package
type (
	ContextWireFormat = wireformat.ContextWireFormat
	DNSRequestWire    = wireformat.DNSRequestWire
	DNSResponseWire   = wireformat.DNSResponseWire
	HTTPRequestWire   = wireformat.HTTPRequestWire
	HTTPResponseWire  = wireformat.HTTPResponseWire
	TCPRequestWire    = wireformat.TCPRequestWire
	TCPResponseWire   = wireformat.TCPResponseWire
	ErrorDetail       = wireformat.ErrorDetail
)

// createContextFromWire creates a new context from the wire format.
func createContextFromWire(parentCtx context.Context, wireCtx ContextWireFormat) (context.Context, context.CancelFunc) {
	if wireCtx.Cancelled {
		slog.Warn("hostfuncs: received already cancelled context from plugin")
		ctx, cancel := context.WithCancel(parentCtx)
		cancel() // Immediately cancel
		return ctx, cancel
	}

	// Apply deadline if present
	if wireCtx.Deadline != nil && !wireCtx.Deadline.IsZero() {
		return context.WithDeadline(parentCtx, *wireCtx.Deadline)
	}

	// Apply timeout if present
	if wireCtx.TimeoutMs > 0 {
		return context.WithTimeout(parentCtx, time.Duration(wireCtx.TimeoutMs)*time.Millisecond)
	}

	return context.WithCancel(parentCtx) // Default to cancellable context
}

// toErrorDetail converts a Go error to our structured ErrorDetail.
func toErrorDetail(err error) *ErrorDetail {
	if err == nil {
		return nil
	}
	// TODO: Expand this to unwrap and categorize errors more granularly
	return &ErrorDetail{
		Message: err.Error(),
		Type:    "internal", // Default type
		Code:    "",
	}
}

// hostWriteResponse writes the JSON response to WASM memory and returns packed ptr+len.
func hostWriteResponse(ctx context.Context, mod api.Module, response interface{}) uint64 {
	data, err := json.Marshal(response)
	if err != nil {
		// Fallback to write a generic error if marshaling fails
		errMsg := fmt.Sprintf("hostfuncs: failed to marshal response: %v", err)
		slog.ErrorContext(ctx, errMsg)
		errResponse := DNSResponseWire{ // Using DNS response as a generic error container for now
			Error: &ErrorDetail{Message: errMsg, Type: "internal"},
		}
		data, _ = json.Marshal(errResponse) // Attempt to marshal fallback
	}

	// Allocate memory in Guest and copy data
	results, err := mod.ExportedFunction("allocate").Call(ctx, uint64(len(data)))
	if err != nil { // Check for error from Guest's allocate function
		slog.ErrorContext(ctx, "hostfuncs: critical - failed to call guest allocate function", "error", err)
		return 0 // Return 0, Host will likely panic or handle this
	}
	ptr := uint32(results[0])

	// Copy data to Guest memory
	mod.Memory().Write(ptr, data)

	// Return packed ptr+len
	return packPtrLen(ptr, uint32(len(data)))
}

// packPtrLen and unpackPtrLen are helper functions consistent with SDK ABI.
func packPtrLen(ptr, length uint32) uint64 {
	return (uint64(ptr) << 32) | uint64(length)
}

func unpackPtrLen(packed uint64) (ptr, length uint32) {
	ptr = uint32(packed >> 32)
	length = uint32(packed)
	return ptr, length
}
