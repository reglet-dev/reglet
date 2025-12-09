package sdk

import (
	"errors"
	"fmt"

	"github.com/whiskeyjimbo/reglet/wireformat"
)

// Config represents the configuration passed to a plugin observation.
type Config map[string]interface{}

// Evidence represents the structured data returned by a plugin observation.
type Evidence struct {
	Status bool                   `json:"status"`
	Data   map[string]interface{} `json:"data,omitempty"`
	Error  *ErrorDetail           `json:"error,omitempty"` // Structured error details
}

// ErrorDetail is re-exported from wireformat for backward compatibility.
// Error Types: "network", "timeout", "config", "panic", "capability", "validation", "internal"
type ErrorDetail = wireformat.ErrorDetail

// Metadata contains information about the plugin.
type Metadata struct {
	Name           string       `json:"name"`
	Version        string       `json:"version"`
	Description    string       `json:"description"`
	SDKVersion     string       `json:"sdk_version"`     // Auto-populated
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

	// Check for custom SDK error types and categorize appropriately
	var (
		netErr    *NetworkError
		dnsErr    *DNSError
		httpErr   *HTTPError
		tcpErr    *TCPError
		timeoutErr *TimeoutError
		capErr    *CapabilityError
		confErr   *ConfigError
		execErr   *ExecError
		schemaErr *SchemaError
		memErr    *MemoryError
		wireErr   *WireFormatError
	)

	switch {
	case errors.As(err, &netErr):
		return &ErrorDetail{
			Message: netErr.Error(),
			Type:    "network",
			Code:    netErr.Operation,
		}
	case errors.As(err, &dnsErr):
		detail := &ErrorDetail{
			Message: dnsErr.Error(),
			Type:    "network",
			Code:    "dns_" + dnsErr.RecordType,
		}
		if dnsErr.Timeout() {
			detail.Type = "timeout"
		}
		return detail
	case errors.As(err, &httpErr):
		detail := &ErrorDetail{
			Message: httpErr.Error(),
			Type:    "network",
			Code:    fmt.Sprintf("http_%d", httpErr.StatusCode),
		}
		if httpErr.Timeout() {
			detail.Type = "timeout"
		}
		return detail
	case errors.As(err, &tcpErr):
		detail := &ErrorDetail{
			Message: tcpErr.Error(),
			Type:    "network",
			Code:    "tcp_connect",
		}
		if tcpErr.Timeout() {
			detail.Type = "timeout"
		}
		return detail
	case errors.As(err, &timeoutErr):
		return &ErrorDetail{
			Message: timeoutErr.Error(),
			Type:    "timeout",
			Code:    timeoutErr.Operation,
		}
	case errors.As(err, &capErr):
		return &ErrorDetail{
			Message: capErr.Error(),
			Type:    "capability",
			Code:    capErr.Required,
		}
	case errors.As(err, &confErr):
		return &ErrorDetail{
			Message: confErr.Error(),
			Type:    "config",
			Code:    confErr.Field,
		}
	case errors.As(err, &execErr):
		return &ErrorDetail{
			Message: execErr.Error(),
			Type:    "exec",
			Code:    fmt.Sprintf("exit_%d", execErr.ExitCode),
		}
	case errors.As(err, &schemaErr):
		return &ErrorDetail{
			Message: schemaErr.Error(),
			Type:    "validation",
			Code:    "schema",
		}
	case errors.As(err, &memErr):
		return &ErrorDetail{
			Message: memErr.Error(),
			Type:    "internal",
			Code:    "memory_limit",
		}
	case errors.As(err, &wireErr):
		return &ErrorDetail{
			Message: wireErr.Error(),
			Type:    "internal",
			Code:    "wire_format",
		}
	default:
		// Generic error - categorize as internal
		return &ErrorDetail{
			Message: err.Error(),
			Type:    "internal",
			Code:    "",
		}
	}
}

// Success creates a successful Evidence with data.
func Success(data map[string]interface{}) Evidence {
	return Evidence{Status: true, Data: data}
}

// Failure creates a failed Evidence with an error.
func Failure(errType, message string) Evidence {
	return Evidence{
		Status: false,
		Error:  &ErrorDetail{Message: message, Type: errType},
	}
}

// ConfigFailure creates a config validation error Evidence.
// Deprecated: Use ConfigError type and proper error handling instead.
func ConfigFailure(err error) Evidence {
	// ToErrorDetail will handle if err is already a *wireformat.ErrorDetail
	return Evidence{
		Status: false,
		Error:  ToErrorDetail(err),
	}
}

// NetworkFailure creates a network error Evidence with wrapped error.
// Deprecated: Use NetworkError type and proper error handling instead.
func NetworkFailure(message string, err error) Evidence {
	return Evidence{
		Status: false,
		Error: &ErrorDetail{
			Message: message,
			Type:    "network",
			Wrapped: ToErrorDetail(err), // Wrapped error is now also processed by ToErrorDetail
		},
	}
}

const (
	// Version of the SDK
	Version = "0.1.0-alpha"
	// MinHostVersion is the minimum compatible Reglet host version.
	MinHostVersion = "0.2.0" // Placeholder, will be determined by host capabilities
)
