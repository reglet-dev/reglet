package sdk

import "fmt"

// Config represents the configuration passed to a plugin observation.
type Config map[string]interface{}

// Evidence represents the structured data returned by a plugin observation.
type Evidence struct {
	Status bool                   `json:"status"`
	Data   map[string]interface{} `json:"data,omitempty"`
	Error  *ErrorDetail           `json:"error,omitempty"` // Structured error details
}

// ErrorDetail provides structured error information.
// Error Types: "network", "timeout", "config", "panic", "capability", "validation", "internal"
type ErrorDetail struct {
	Message string       `json:"message"`
	Type    string       `json:"type"`     // "network", "timeout", "config", "panic", "capability", "validation", "internal"
	Code    string       `json:"code"`     // "ECONNREFUSED", "ETIMEDOUT", etc.
	Wrapped *ErrorDetail `json:"wrapped,omitempty"`
	Stack   []byte       `json:"stack,omitempty"` // Stack trace for panic errors
}

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
// This function can be expanded to unwrap errors and categorize them into specific types/codes.
func ToErrorDetail(err error) *ErrorDetail {
	if err == nil {
		return nil
	}
	// For now, a simple conversion. Can be expanded to unwrap errors and categorize.
	return &ErrorDetail{
		Message: err.Error(),
		Type:    "internal", // Default type, can be refined later
		Code:    "",
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

// ConfigError creates a config validation error Evidence.
func ConfigError(err error) Evidence {
	return Evidence{
		Status: false,
		Error:  &ErrorDetail{Message: err.Error(), Type: "config"},
	}
}

// NetworkError creates a network error Evidence with wrapped error.
func NetworkError(message string, err error) Evidence {
	return Evidence{
		Status: false,
		Error: &ErrorDetail{
			Message: message,
			Type:    "network",
			Wrapped: ToErrorDetail(err),
		},
	}
}

// Error method to make ErrorDetail implement the error interface.
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

const (
	// Version of the SDK
	Version = "0.1.0-alpha"
	// MinHostVersion is the minimum compatible Reglet host version.
	MinHostVersion = "0.2.0" // Placeholder, will be determined by host capabilities
)
