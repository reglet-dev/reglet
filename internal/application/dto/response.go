package dto

import (
	"time"

	"github.com/whiskeyjimbo/reglet/internal/domain/capabilities"
	"github.com/whiskeyjimbo/reglet/internal/domain/execution"
)

// CheckProfileResponse contains the result of checking a profile.
type CheckProfileResponse struct {
	// ExecutionResult contains the detailed execution results
	ExecutionResult *execution.ExecutionResult

	// Metadata contains response metadata
	Metadata ResponseMetadata

	// Diagnostics contains additional diagnostic information
	Diagnostics Diagnostics
}

// ResponseMetadata contains metadata about the response.
type ResponseMetadata struct {
	// RequestID from the original request
	RequestID string

	// ProcessedAt is when the request was processed
	ProcessedAt time.Time

	// Duration is how long the request took
	Duration time.Duration
}

// Diagnostics contains diagnostic information about execution.
type Diagnostics struct {
	// Warnings are non-fatal issues encountered
	Warnings []string

	// Capabilities shows what capabilities were involved
	Capabilities CapabilityDiagnostics
}

// CapabilityDiagnostics contains capability-related diagnostics.
type CapabilityDiagnostics struct {
	// Required capabilities by plugin
	Required map[string][]capabilities.Capability

	// Granted capabilities by plugin
	Granted map[string][]capabilities.Capability
}

// LoadProfileResponse contains the result of loading a profile.
type LoadProfileResponse struct {
	// Profile is the loaded and validated profile
	// Note: We don't expose the profile entity directly in DTO,
	// but for now we'll keep it simple. In a strict hexagonal architecture,
	// this would be a separate DTO.
	ProfilePath string
	Success     bool
}

// CollectCapabilitiesResponse contains the result of capability collection.
type CollectCapabilitiesResponse struct {
	// Required capabilities by plugin name
	Required map[string][]capabilities.Capability

	// Granted capabilities by plugin name
	Granted map[string][]capabilities.Capability
}

// ExecuteProfileResponse contains the result of profile execution.
type ExecuteProfileResponse struct {
	// ExecutionResult is the domain execution result
	ExecutionResult *execution.ExecutionResult
}
