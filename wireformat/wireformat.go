// Package wireformat defines the JSON wire format structures for communication
// between the WASM host and guest (plugins). These types must remain stable
// and backward compatible as they define the ABI contract.
package wireformat

import (
	"fmt"
	"time"
)

// ContextWireFormat is the JSON wire format for context.Context propagation.
type ContextWireFormat struct {
	Deadline  *time.Time `json:"deadline,omitempty"`
	TimeoutMs int64      `json:"timeout_ms,omitempty"`
	RequestID string     `json:"request_id,omitempty"` // For log correlation
	Cancelled bool       `json:"cancelled,omitempty"`  // True if context is already cancelled
}

// DNSRequestWire is the JSON wire format for a DNS lookup request from Guest to Host.
type DNSRequestWire struct {
	Context    ContextWireFormat `json:"context"`
	Hostname   string            `json:"hostname"`
	Type       string            `json:"type"`                 // "A", "AAAA", "CNAME", "MX", "TXT", "NS"
	Nameserver string            `json:"nameserver,omitempty"` // Optional: "host:port"
}

// DNSResponseWire is the JSON wire format for a DNS lookup response from Host to Guest.
type DNSResponseWire struct {
	Records   []string       `json:"records,omitempty"`
	MXRecords []MXRecordWire `json:"mx_records,omitempty"`
	Error     *ErrorDetail   `json:"error,omitempty"` // Structured error
}

// MXRecordWire represents a single MX record.
type MXRecordWire struct {
	Host string `json:"host"`
	Pref uint16 `json:"pref"`
}

// HTTPRequestWire is the JSON wire format for an HTTP request from Guest to Host.
type HTTPRequestWire struct {
	Context ContextWireFormat   `json:"context"`
	Method  string              `json:"method"`
	URL     string              `json:"url"`
	Headers map[string][]string `json:"headers,omitempty"`
	Body    string              `json:"body,omitempty"` // Base64 encoded for binary, or plain string
	// TimeoutMs is implied by Context.TimeoutMs
}

// HTTPResponseWire is the JSON wire format for an HTTP response from Host to Guest.
type HTTPResponseWire struct {
	StatusCode    int                 `json:"status_code"`
	Headers       map[string][]string `json:"headers,omitempty"`
	Body          string              `json:"body,omitempty"`           // Base64 encoded for binary, or plain string
	BodyTruncated bool                `json:"body_truncated,omitempty"` // True if response body exceeded size limit
	Error         *ErrorDetail        `json:"error,omitempty"`          // Structured error
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
	Connected       bool         `json:"connected"`
	Address         string       `json:"address,omitempty"`
	RemoteAddr      string       `json:"remote_addr,omitempty"`
	LocalAddr       string       `json:"local_addr,omitempty"`
	ResponseTimeMs  int64        `json:"response_time_ms,omitempty"`
	TLS             bool         `json:"tls,omitempty"`
	TLSVersion      string       `json:"tls_version,omitempty"`
	TLSCipherSuite  string       `json:"tls_cipher_suite,omitempty"`
	TLSServerName   string       `json:"tls_server_name,omitempty"`
	TLSCertSubject  string       `json:"tls_cert_subject,omitempty"`
	TLSCertIssuer   string       `json:"tls_cert_issuer,omitempty"`
	TLSCertNotAfter *time.Time   `json:"tls_cert_not_after,omitempty"`
	Error           *ErrorDetail `json:"error,omitempty"` // Structured error
}

// SMTPRequestWire is the JSON wire format for an SMTP connection request from Guest to Host.
type SMTPRequestWire struct {
	Context   ContextWireFormat `json:"context"`
	Host      string            `json:"host"`
	Port      string            `json:"port"`
	TimeoutMs int               `json:"timeout_ms,omitempty"` // Optional timeout in milliseconds
	TLS       bool              `json:"tls"`                  // Whether to use TLS (SMTPS on port 465)
	StartTLS  bool              `json:"starttls"`             // Whether to use STARTTLS (upgrade to TLS)
}

// SMTPResponseWire is the JSON wire format for an SMTP connection response from Host to Guest.
type SMTPResponseWire struct {
	Connected      bool         `json:"connected"`
	Address        string       `json:"address,omitempty"`
	Banner         string       `json:"banner,omitempty"`          // SMTP banner message
	ResponseTimeMs int64        `json:"response_time_ms,omitempty"`
	TLS            bool         `json:"tls,omitempty"`
	TLSVersion     string       `json:"tls_version,omitempty"`
	TLSCipherSuite string       `json:"tls_cipher_suite,omitempty"`
	TLSServerName  string       `json:"tls_server_name,omitempty"`
	Error          *ErrorDetail `json:"error,omitempty"` // Structured error
}

// ExecRequestWire is the JSON wire format for an exec request from Guest to Host.
type ExecRequestWire struct {
	Context ContextWireFormat `json:"context"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Dir     string            `json:"dir,omitempty"`
	Env     []string          `json:"env,omitempty"`
}

// ExecResponseWire is the JSON wire format for an exec response from Host to Guest.
type ExecResponseWire struct {
	Stdout     string       `json:"stdout"`
	Stderr     string       `json:"stderr"`
	ExitCode   int          `json:"exit_code"`
	DurationMs int64        `json:"duration_ms,omitempty"` // Execution duration in milliseconds
	IsTimeout  bool         `json:"is_timeout,omitempty"`  // True if command timed out
	Error      *ErrorDetail `json:"error,omitempty"`
}

// ErrorDetail provides structured error information, consistent across host and SDK.
// Error Types: "network", "timeout", "config", "panic", "capability", "validation", "internal"
type ErrorDetail struct {
	Message    string       `json:"message"`
	Type       string       `json:"type"`                   // "network", "timeout", "config", "panic", "capability", "validation", "internal"
	Code       string       `json:"code"`                   // "ECONNREFUSED", "ETIMEDOUT", etc.
	IsTimeout  bool         `json:"is_timeout,omitempty"`   // For network errors
	IsNotFound bool         `json:"is_not_found,omitempty"` // For network/DNS errors
	Wrapped    *ErrorDetail `json:"wrapped,omitempty"`
	Stack      []byte       `json:"stack,omitempty"` // Stack trace for panic errors (SDK only)
}

// Error implements the error interface for ErrorDetail.
func (e *ErrorDetail) Error() string {
	if e == nil {
		return ""
	}
	msg := e.Message
	if e.Type != "" && e.Type != "internal" {
		msg = fmt.Sprintf("%s: %s", e.Type, msg)
	}
	if e.Code != "" {
		msg = fmt.Sprintf("%s [%s]", msg, e.Code)
	}
	if e.Wrapped != nil {
		msg = fmt.Sprintf("%s: %v", msg, e.Wrapped.Error())
	}
	return msg
}
