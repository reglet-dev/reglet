// Package sdk provides core types and functions for building Reglet plugins.
package sdk

import (
	"errors"
	"fmt"
	"time" // Added for Timestamp

	"github.com/reglet-dev/reglet/wireformat"
)

// Config represents the configuration passed to a plugin observation.
type Config map[string]interface{}

// Evidence represents the structured data returned by a plugin observation.
// This struct directly mirrors the WIT 'evidence' record for direct mapping
// across the WebAssembly boundary.
type Evidence struct {
	Status    bool                   // Corresponds to WIT 'status'
	Error     *ErrorDetail           // Corresponds to WIT 'error'
	Timestamp time.Time              // Corresponds to WIT 'timestamp'
	Data      map[string]interface{} // Corresponds to WIT 'data'
	Raw       *string                // Corresponds to WIT 'raw'
}

// ErrorDetail is re-exported from wireformat for backward compatibility.
// Error Types: "network", "timeout", "config", "panic", "capability", "validation", "internal"
type ErrorDetail = wireformat.ErrorDetail

// Metadata contains information about the plugin.
type Metadata struct {
	Name           string       `json:"name"`
	Version        string       `json:"version"`
	Description    string       `json:"description"`
	SDKVersion     string       `json:"sdk_version"`      // Auto-populated
	MinHostVersion string       `json:"min_host_version"` // Minimum compatible host
	Capabilities   []Capability `json:"capabilities"`
}

// Capability describes a permission required by the plugin.
type Capability struct {
	Kind    string `json:"kind"`
	Pattern string `json:"pattern"`
}

// ToErrorDetail converts a Go error to our structured ErrorDetail.
// This function recognizes custom error types and categorizes them appropriately.
func ToErrorDetail(err error) *ErrorDetail {
	if err == nil {
		return nil
	}

	// If the error is already a *wireformat.ErrorDetail, use it directly.
	var wfError *wireformat.ErrorDetail
	if errors.As(err, &wfError) {
		return wfError
	}

	// Try each custom error type
	if detail := convertNetworkError(err); detail != nil {
		return detail
	}
	if detail := convertDNSError(err); detail != nil {
		return detail
	}
	if detail := convertHTTPError(err); detail != nil {
		return detail
	}
	if detail := convertTCPError(err); detail != nil {
		return detail
	}
	if detail := convertTimeoutError(err); detail != nil {
		return detail
	}
	if detail := convertCapabilityError(err); detail != nil {
		return detail
	}
	if detail := convertConfigError(err); detail != nil {
		return detail
	}
	if detail := convertExecError(err); detail != nil {
		return detail
	}
	if detail := convertSchemaError(err); detail != nil {
		return detail
	}
	if detail := convertMemoryError(err); detail != nil {
		return detail
	}
	if detail := convertWireFormatError(err); detail != nil {
		return detail
	}

	// Generic error - categorize as internal
	return &ErrorDetail{
		Message: err.Error(),
		Type:    "internal",
		Code:    "",
	}
}

func convertNetworkError(err error) *ErrorDetail {
	var netErr *NetworkError
	if errors.As(err, &netErr) {
		return &ErrorDetail{Message: netErr.Error(), Type: "network", Code: netErr.Operation}
	}
	return nil
}

func convertDNSError(err error) *ErrorDetail {
	var dnsErr *DNSError
	if errors.As(err, &dnsErr) {
		detail := &ErrorDetail{Message: dnsErr.Error(), Type: "network", Code: "dns_" + dnsErr.RecordType}
		if dnsErr.Timeout() {
			detail.Type = "timeout"
		}
		return detail
	}
	return nil
}

func convertHTTPError(err error) *ErrorDetail {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		detail := &ErrorDetail{Message: httpErr.Error(), Type: "network", Code: fmt.Sprintf("http_%d", httpErr.StatusCode)}
		if httpErr.Timeout() {
			detail.Type = "timeout"
		}
		return detail
	}
	return nil
}

func convertTCPError(err error) *ErrorDetail {
	var tcpErr *TCPError
	if errors.As(err, &tcpErr) {
		detail := &ErrorDetail{Message: tcpErr.Error(), Type: "network", Code: "tcp_connect"}
		if tcpErr.Timeout() {
			detail.Type = "timeout"
		}
		return detail
	}
	return nil
}

func convertTimeoutError(err error) *ErrorDetail {
	var timeoutErr *TimeoutError
	if errors.As(err, &timeoutErr) {
		return &ErrorDetail{Message: timeoutErr.Error(), Type: "timeout", Code: timeoutErr.Operation}
	}
	return nil
}

func convertCapabilityError(err error) *ErrorDetail {
	var capErr *CapabilityError
	if errors.As(err, &capErr) {
		return &ErrorDetail{Message: capErr.Error(), Type: "capability", Code: capErr.Required}
	}
	return nil
}

func convertConfigError(err error) *ErrorDetail {
	var confErr *ConfigError
	if errors.As(err, &confErr) {
		return &ErrorDetail{Message: confErr.Error(), Type: "config", Code: confErr.Field}
	}
	return nil
}

func convertExecError(err error) *ErrorDetail {
	var execErr *ExecError
	if errors.As(err, &execErr) {
		return &ErrorDetail{Message: execErr.Error(), Type: "exec", Code: fmt.Sprintf("exit_%d", execErr.ExitCode)}
	}
	return nil
}

func convertSchemaError(err error) *ErrorDetail {
	var schemaErr *SchemaError
	if errors.As(err, &schemaErr) {
		return &ErrorDetail{Message: schemaErr.Error(), Type: "validation", Code: "schema"}
	}
	return nil
}

func convertMemoryError(err error) *ErrorDetail {
	var memErr *MemoryError
	if errors.As(err, &memErr) {
		return &ErrorDetail{Message: memErr.Error(), Type: "internal", Code: "memory_limit"}
	}
	return nil
}

func convertWireFormatError(err error) *ErrorDetail {
	var wireErr *WireFormatError
	if errors.As(err, &wireErr) {
		return &ErrorDetail{Message: wireErr.Error(), Type: "internal", Code: "wire_format"}
	}
	return nil
}

// Success creates a successful Evidence with data.
func Success(data map[string]interface{}) Evidence {
	return Evidence{Status: true, Data: data, Timestamp: time.Now()}
}

// Failure creates a failed Evidence with an error.
func Failure(errType, message string) Evidence {
	return Evidence{
		Status:    false,
		Error:     &ErrorDetail{Message: message, Type: errType},
		Timestamp: time.Now(),
	}
}

const (
	// Version of the SDK
	Version = "0.1.0-alpha"
	// MinHostVersion is the minimum compatible Reglet host version.
	MinHostVersion = "0.2.0" // Placeholder, will be determined by host capabilities
)
