// Package sdk provides the Reglet plugin SDK for building WASM compliance check plugins.
//
// This package includes custom error types for better error handling and inspection.
// All error types support error unwrapping via errors.As() and errors.Is().
package sdk

import (
	"fmt"
	"time"
)

// NetworkError represents a network operation failure.
type NetworkError struct {
	Operation string // "dns_lookup", "http_request", "tcp_connect", etc.
	Target    string // Hostname, URL, or address
	Err       error  // Underlying error
}

func (e *NetworkError) Error() string {
	if e.Target != "" {
		return fmt.Sprintf("network %s failed for %s: %v", e.Operation, e.Target, e.Err)
	}
	return fmt.Sprintf("network %s failed: %v", e.Operation, e.Err)
}

func (e *NetworkError) Unwrap() error {
	return e.Err
}

// TimeoutError represents a timeout during an operation.
type TimeoutError struct {
	Operation string        // "dns_lookup", "http_request", "exec_command", etc.
	Duration  time.Duration // How long we waited before timing out
	Target    string        // Optional: what we were trying to reach
}

func (e *TimeoutError) Error() string {
	if e.Target != "" {
		return fmt.Sprintf("%s timeout after %v (target: %s)", e.Operation, e.Duration, e.Target)
	}
	return fmt.Sprintf("%s timeout after %v", e.Operation, e.Duration)
}

func (e *TimeoutError) Timeout() bool {
	return true
}

// CapabilityError represents a capability check failure.
type CapabilityError struct {
	Required string // Required capability (e.g., "network:outbound", "exec")
	Pattern  string // Optional: specific pattern that was denied
}

func (e *CapabilityError) Error() string {
	if e.Pattern != "" {
		return fmt.Sprintf("missing capability: %s (pattern: %s)", e.Required, e.Pattern)
	}
	return fmt.Sprintf("missing capability: %s", e.Required)
}

// ConfigError represents a configuration validation error.
type ConfigError struct {
	Field string // Field name that failed validation
	Err   error  // Underlying validation error
}

func (e *ConfigError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("config validation failed for field '%s': %v", e.Field, e.Err)
	}
	return fmt.Sprintf("config validation failed: %v", e.Err)
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}

// ExecError represents a command execution error.
type ExecError struct {
	Command  string // Command that was executed
	ExitCode int    // Exit code if command ran
	Stderr   string // Standard error output
	Err      error  // Underlying error (if command didn't run)
}

func (e *ExecError) Error() string {
	if e.Err != nil {
		// Command didn't run (not found, permission denied, etc.)
		return fmt.Sprintf("failed to execute '%s': %v", e.Command, e.Err)
	}
	// Command ran but failed
	if e.Stderr != "" {
		return fmt.Sprintf("command '%s' exited with code %d: %s", e.Command, e.ExitCode, e.Stderr)
	}
	return fmt.Sprintf("command '%s' exited with code %d", e.Command, e.ExitCode)
}

func (e *ExecError) Unwrap() error {
	return e.Err
}

// DNSError represents a DNS lookup failure.
type DNSError struct {
	Hostname   string // Hostname that failed to resolve
	RecordType string // Type of record (A, AAAA, MX, etc.)
	Nameserver string // Optional: specific nameserver used
	Err        error  // Underlying error
}

func (e *DNSError) Error() string {
	if e.Nameserver != "" {
		return fmt.Sprintf("dns lookup for %s (%s) via %s failed: %v",
			e.Hostname, e.RecordType, e.Nameserver, e.Err)
	}
	return fmt.Sprintf("dns lookup for %s (%s) failed: %v", e.Hostname, e.RecordType, e.Err)
}

func (e *DNSError) Unwrap() error {
	return e.Err
}

func (e *DNSError) Timeout() bool {
	// Check if underlying error is a timeout
	if t, ok := e.Err.(interface{ Timeout() bool }); ok {
		return t.Timeout()
	}
	return false
}

// HTTPError represents an HTTP request failure.
type HTTPError struct {
	Method     string // HTTP method (GET, POST, etc.)
	URL        string // Request URL
	StatusCode int    // HTTP status code (0 if request failed before receiving response)
	Err        error  // Underlying error
}

func (e *HTTPError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("http %s %s failed with status %d: %v", e.Method, e.URL, e.StatusCode, e.Err)
	}
	return fmt.Sprintf("http %s %s failed: %v", e.Method, e.URL, e.Err)
}

func (e *HTTPError) Unwrap() error {
	return e.Err
}

func (e *HTTPError) Timeout() bool {
	// Check if underlying error is a timeout
	if t, ok := e.Err.(interface{ Timeout() bool }); ok {
		return t.Timeout()
	}
	return false
}

// TCPError represents a TCP connection failure.
type TCPError struct {
	Network string // "tcp", "tcp4", or "tcp6"
	Address string // Target address (host:port)
	Err     error  // Underlying error
}

func (e *TCPError) Error() string {
	return fmt.Sprintf("tcp connect to %s (%s) failed: %v", e.Address, e.Network, e.Err)
}

func (e *TCPError) Unwrap() error {
	return e.Err
}

func (e *TCPError) Timeout() bool {
	// Check if underlying error is a timeout
	if t, ok := e.Err.(interface{ Timeout() bool }); ok {
		return t.Timeout()
	}
	return false
}

// SchemaError represents a schema generation or validation error.
type SchemaError struct {
	Type string // Go type that failed schema generation
	Err  error  // Underlying error
}

func (e *SchemaError) Error() string {
	if e.Type != "" {
		return fmt.Sprintf("schema error for type %s: %v", e.Type, e.Err)
	}
	return fmt.Sprintf("schema error: %v", e.Err)
}

func (e *SchemaError) Unwrap() error {
	return e.Err
}

// MemoryError represents a memory allocation failure.
type MemoryError struct {
	Requested int // Requested allocation size
	Current   int // Current total allocated
	Limit     int // Maximum allowed
}

func (e *MemoryError) Error() string {
	return fmt.Sprintf("memory allocation failed: requested %d bytes, current %d bytes, limit %d bytes",
		e.Requested, e.Current, e.Limit)
}

// WireFormatError represents a wire format encoding/decoding error.
type WireFormatError struct {
	Operation string // "marshal" or "unmarshal"
	Type      string // Type being encoded/decoded
	Err       error  // Underlying error
}

func (e *WireFormatError) Error() string {
	return fmt.Sprintf("wire format %s failed for %s: %v", e.Operation, e.Type, e.Err)
}

func (e *WireFormatError) Unwrap() error {
	return e.Err
}
