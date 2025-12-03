package net

import (
	"context"
	"time"

	"github.com/whiskeyjimbo/reglet/sdk" // For sdk.ErrorDetail
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
	Type       string            `json:"type"` // "A", "AAAA", "CNAME", "MX", "TXT", "NS"
	Nameserver string            `json:"nameserver,omitempty"` // Optional: "host:port"
}

// DNSResponseWire is the JSON wire format for a DNS lookup response from Host to Guest.
type DNSResponseWire struct {
	Records []string         `json:"records,omitempty"`
	Error   *sdk.ErrorDetail `json:"error,omitempty"` // Structured error
}

// TCPRequestWire is the JSON wire format for a TCP connection request from Guest to Host.
type TCPRequestWire struct {
	Context   ContextWireFormat `json:"context"`
	Host      string            `json:"host"`
	Port      string            `json:"port"`
	TimeoutMs int               `json:"timeout_ms,omitempty"`
	TLS       bool              `json:"tls"`
}

// TCPResponseWire is the JSON wire format for a TCP connection response from Host to Guest.
type TCPResponseWire struct {
	Connected      bool             `json:"connected"`
	Address        string           `json:"address,omitempty"`
	RemoteAddr     string           `json:"remote_addr,omitempty"`
	LocalAddr      string           `json:"local_addr,omitempty"`
	ResponseTimeMs int64            `json:"response_time_ms,omitempty"`
	TLS            bool             `json:"tls,omitempty"`
	TLSVersion     string           `json:"tls_version,omitempty"`
	TLSCipherSuite string           `json:"tls_cipher_suite,omitempty"`
	TLSServerName  string           `json:"tls_server_name,omitempty"`
	TLSCertSubject string           `json:"tls_cert_subject,omitempty"`
	TLSCertIssuer  string           `json:"tls_cert_issuer,omitempty"`
	Error          *sdk.ErrorDetail `json:"error,omitempty"`
}


// createContextWireFormat extracts relevant info from a Go context into the wire format.
func createContextWireFormat(ctx context.Context) ContextWireFormat {
	wire := ContextWireFormat{}

	// Handle deadline
	if deadline, ok := ctx.Deadline(); ok {
		wire.Deadline = &deadline
		wire.TimeoutMs = time.Until(deadline).Milliseconds()
	}

	// Handle cancellation
	select {
	case <-ctx.Done():
		wire.Cancelled = true
	default:
		// Not cancelled yet
	}

	// TODO: Extract request_id if available in context values
	// Example: if reqID, ok := ctx.Value("request_id").(string); ok { wire.RequestID = reqID }

	return wire
}
