package hostfuncs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/tetratelabs/wazero/api"
)

// ContextWireFormat is the JSON wire format for context.Context propagation.
type ContextWireFormat struct {
	Deadline   *time.Time `json:"deadline,omitempty"`
	TimeoutMs  int64      `json:"timeout_ms,omitempty"`
	RequestID  string     `json:"request_id,omitempty"` // For log correlation
	Cancelled  bool       `json:"cancelled,omitempty"`  // True if context is already cancelled
}

// DNSRequestWire is the JSON wire format for a DNS lookup request from Guest to Host.
type DNSRequestWire struct {
	Context    ContextWireFormat `json:"context"`
	Hostname   string            `json:"hostname"`
	Type       string            `json:"type"` // "A", "AAAA", "CNAME", "MX", "TXT", "NS"
	Nameserver string            `json:"nameserver,omitempty"` // Optional: "host:port"
}

// DNSResponseWire is the JSON wire format for a DNS lookup response from Host to Guest.
type DNSResponseWire struct {
	Records []string     `json:"records,omitempty"`
	Error   *ErrorDetail `json:"error,omitempty"` // Structured error
}

// HTTPRequestWire is the JSON wire format for an HTTP request from Guest to Host.
type HTTPRequestWire struct {
	Context ContextWireFormat `json:"context"`
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string][]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"` // Base64 encoded for binary, or plain string
	// TimeoutMs is implied by Context.TimeoutMs
}

// HTTPResponseWire is the JSON wire format for an HTTP response from Host to Guest.
type HTTPResponseWire struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string][]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"` // Base64 encoded for binary, or plain string
	Error      *ErrorDetail      `json:"error,omitempty"` // Structured error
}

// TCPRequestWire is the JSON wire format for a TCP connection request from Guest to Host.
type TCPRequestWire struct {
	Context   ContextWireFormat `json:"context"`
	Host      string            `json:"host"`
	Port      string            `json:"port"`
	TimeoutMs int               `json:"timeout_ms,omitempty"` // Optional timeout in milliseconds
	TLS       bool              `json:"tls"`                  // Whether to use TLS
}

// TCPResponseWire is the JSON wire format for a TCP connection response from Host to Guest.
type TCPResponseWire struct {
	Connected      bool              `json:"connected"`
	Address        string            `json:"address,omitempty"`
	RemoteAddr     string            `json:"remote_addr,omitempty"`
	LocalAddr      string            `json:"local_addr,omitempty"`
	ResponseTimeMs int64             `json:"response_time_ms,omitempty"`
	TLS            bool              `json:"tls,omitempty"`
	TLSVersion     string            `json:"tls_version,omitempty"`
	TLSCipherSuite string            `json:"tls_cipher_suite,omitempty"`
	TLSServerName  string            `json:"tls_server_name,omitempty"`
	TLSCertSubject string            `json:"tls_cert_subject,omitempty"`
	TLSCertIssuer  string            `json:"tls_cert_issuer,omitempty"`
	Error          *ErrorDetail      `json:"error,omitempty"` // Structured error
}

// ErrorDetail provides structured error information, consistent with SDK's ErrorDetail.
type ErrorDetail struct {
	Message string       `json:"message"`
	Type    string       `json:"type"`
	Code    string       `json:"code"`
	Wrapped *ErrorDetail `json:"wrapped,omitempty"`
}

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
