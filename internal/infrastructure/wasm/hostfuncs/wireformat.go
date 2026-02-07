package hostfuncs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/reglet-dev/reglet-abi/hostfunc"
	"github.com/tetratelabs/wazero/api"
)

type (
	// ContextWireFormat is an alias for hostfunc.ContextWire
	ContextWireFormat = hostfunc.ContextWire
	// DNSRequestWire is an alias for hostfunc.DNSRequest
	DNSRequestWire = hostfunc.DNSRequest
	// DNSResponseWire is an alias for hostfunc.DNSResponse
	DNSResponseWire = hostfunc.DNSResponse
	// HTTPRequestWire is an alias for hostfunc.HTTPRequest
	HTTPRequestWire = hostfunc.HTTPRequest
	// HTTPResponseWire is an alias for hostfunc.HTTPResponse
	HTTPResponseWire = hostfunc.HTTPResponse
	// TCPRequestWire is an alias for hostfunc.TCPRequest
	TCPRequestWire = hostfunc.TCPRequest
	// TCPResponseWire is an alias for hostfunc.TCPResponse
	TCPResponseWire = hostfunc.TCPResponse
	// SMTPRequestWire is an alias for hostfunc.SMTPRequest
	SMTPRequestWire = hostfunc.SMTPRequest
	// SMTPResponseWire is an alias for hostfunc.SMTPResponse
	SMTPResponseWire = hostfunc.SMTPResponse
	// ExecRequestWire is an alias for hostfunc.ExecRequest
	ExecRequestWire = hostfunc.ExecRequest
	// ExecResponseWire is an alias for hostfunc.ExecResponse
	ExecResponseWire = hostfunc.ExecResponse
	// ErrorDetail is an alias for hostfunc.ErrorDetail
	ErrorDetail = hostfunc.ErrorDetail
	// MXRecordWire is an alias for hostfunc.MXRecord
	MXRecordWire = hostfunc.MXRecord
	// LogMessageWire is an alias for hostfunc.LogMessage
	LogMessageWire = hostfunc.LogMessage
	// LogAttrWire is an alias for hostfunc.LogAttr
	LogAttrWire = hostfunc.LogAttr
)

// createContextFromWire creates a new context from the wire format.
func createContextFromWire(parentCtx context.Context, wireCtx ContextWireFormat) (context.Context, context.CancelFunc) {
	if wireCtx.Canceled {
		slog.Warn("hostfuncs: received already canceled context from plugin")
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
	ptr := uint32(results[0]) //nolint:gosec // G115: WASM32 pointers are always 32-bit

	// Copy data to Guest memory
	mod.Memory().Write(ptr, data)

	// Return packed ptr+len
	return packPtrLen(ptr, uint32(len(data))) //nolint:gosec // G115: WASM memory allocations are bounded to 4GB
}

// packPtrLen and unpackPtrLen are helper functions consistent with SDK ABI.
func packPtrLen(ptr, length uint32) uint64 {
	return (uint64(ptr) << 32) | uint64(length)
}

func unpackPtrLen(packed uint64) (ptr, length uint32) {
	ptr = uint32(packed >> 32) //nolint:gosec // G115: Packed format stores 32-bit values
	length = uint32(packed)    //nolint:gosec // G115: Packed format stores 32-bit values
	return ptr, length
}
