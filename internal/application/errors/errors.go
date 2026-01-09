// Package apperrors defines application-level error types.
package apperrors

import (
	"fmt"

	"github.com/reglet-dev/reglet/internal/domain/capabilities"
)

// ValidationError indicates profile or filter validation failed.
type ValidationError struct {
	Field   string   // Field that failed validation
	Message string   // Error message
	Details []string // Additional details
}

func (e *ValidationError) Error() string {
	if len(e.Details) == 0 {
		return fmt.Sprintf("validation failed: %s: %s", e.Field, e.Message)
	}
	return fmt.Sprintf("validation failed: %s: %s (%d issues)", e.Field, e.Message, len(e.Details))
}

// NewValidationError creates a new validation error.
func NewValidationError(field, message string, details ...string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
		Details: details,
	}
}

// CapabilityError indicates capability permission issue.
type CapabilityError struct {
	Reason   string
	Required []capabilities.Capability
}

func (e *CapabilityError) Error() string {
	return fmt.Sprintf("capability error: %s (%d capabilities required)", e.Reason, len(e.Required))
}

// NewCapabilityError creates a new capability error.
func NewCapabilityError(reason string, required []capabilities.Capability) *CapabilityError {
	return &CapabilityError{
		Required: required,
		Reason:   reason,
	}
}

// ExecutionError indicates execution failed (not validation).
type ExecutionError struct {
	Cause     error
	ControlID string
	Message   string
}

func (e *ExecutionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("execution failed for control %s: %s: %v", e.ControlID, e.Message, e.Cause)
	}
	return fmt.Sprintf("execution failed for control %s: %s", e.ControlID, e.Message)
}

func (e *ExecutionError) Unwrap() error {
	return e.Cause
}

// NewExecutionError creates a new execution error.
func NewExecutionError(controlID, message string, cause error) *ExecutionError {
	return &ExecutionError{
		ControlID: controlID,
		Message:   message,
		Cause:     cause,
	}
}

// ConfigurationError indicates system config or setup issue.
type ConfigurationError struct {
	Cause   error
	Aspect  string
	Message string
}

func (e *ConfigurationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("configuration error (%s): %s: %v", e.Aspect, e.Message, e.Cause)
	}
	return fmt.Sprintf("configuration error (%s): %s", e.Aspect, e.Message)
}

func (e *ConfigurationError) Unwrap() error {
	return e.Cause
}

// NewConfigurationError creates a new configuration error.
func NewConfigurationError(aspect, message string, cause error) *ConfigurationError {
	return &ConfigurationError{
		Aspect:  aspect,
		Message: message,
		Cause:   cause,
	}
}
