package hostfuncs

import (
	"context"
	"encoding/json"
	"errors" // New import
	"fmt"
	"log/slog"
	"net" // New import
	"time"

	"github.com/reglet-dev/reglet/wireformat"
	"github.com/tetratelabs/wazero/api"
)

type (
	// ContextWireFormat is a re-export of wireformat.ContextWireFormat
	ContextWireFormat = wireformat.ContextWireFormat
	// DNSRequestWire is a re-export of wireformat.DNSRequestWire
	DNSRequestWire = wireformat.DNSRequestWire
	// DNSResponseWire is a re-export of wireformat.DNSResponseWire
	DNSResponseWire = wireformat.DNSResponseWire
	// HTTPRequestWire is a re-export of wireformat.HTTPRequestWire
	HTTPRequestWire = wireformat.HTTPRequestWire
	// HTTPResponseWire is a re-export of wireformat.HTTPResponseWire
	HTTPResponseWire = wireformat.HTTPResponseWire
	// TCPRequestWire is a re-export of wireformat.TCPRequestWire
	TCPRequestWire = wireformat.TCPRequestWire
	// TCPResponseWire is a re-export of wireformat.TCPResponseWire
	TCPResponseWire = wireformat.TCPResponseWire
	// SMTPRequestWire is a re-export of wireformat.SMTPRequestWire
	SMTPRequestWire = wireformat.SMTPRequestWire
	// SMTPResponseWire is a re-export of wireformat.SMTPResponseWire
	SMTPResponseWire = wireformat.SMTPResponseWire
	// ExecRequestWire is a re-export of wireformat.ExecRequestWire
	ExecRequestWire = wireformat.ExecRequestWire
	// ExecResponseWire is a re-export of wireformat.ExecResponseWire
	ExecResponseWire = wireformat.ExecResponseWire
	// ErrorDetail is a re-export of wireformat.ErrorDetail
	ErrorDetail = wireformat.ErrorDetail
	// MXRecordWire is a re-export of wireformat.MXRecordWire
	MXRecordWire = wireformat.MXRecordWire
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

// toErrorDetail converts a Go error to our structured ErrorDetail.
func toErrorDetail(err error) *ErrorDetail {
	if err == nil {
		return nil
	}

	detail := &ErrorDetail{
		Message: err.Error(),
		Type:    "internal", // Default type
		Code:    "",
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		detail.Type = "network" // DNS errors are network errors
		if dnsErr.IsTimeout {
			detail.Type = "timeout"
			detail.IsTimeout = true
		}
		if dnsErr.IsNotFound {
			detail.IsNotFound = true
			// Could add specific code like "NXDOMAIN" if needed
		}
		// Consider other net.DNSError flags if relevant
	}

	// TODO: Expand this to unwrap and categorize errors more granularly for other types of errors
	return detail
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
