package dto

import (
	"time"

	"github.com/reglet-dev/reglet/internal/domain/capability"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/execution"
)

// CheckProfileResponse contains the result of checking a profile.
type CheckProfileResponse struct {
	Diagnostics     Diagnostics
	ExecutionResult *execution.ExecutionResult
	Metadata        ResponseMetadata
}

// ResponseMetadata contains metadata about the response.
type ResponseMetadata struct {
	ProcessedAt time.Time
	RequestID   string
	Duration    time.Duration
}

// Diagnostics contains diagnostic information about execution.
type Diagnostics struct {
	Capabilities CapabilityDiagnostics
	Warnings     []string
}

// CapabilityDiagnostics contains capability-related diagnostics.
type CapabilityDiagnostics struct {
	// Required capabilities by plugin
	Required map[string]capability.GrantSet

	// Granted capabilities by plugin
	Granted map[string]capability.GrantSet
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
	Required map[string]capability.GrantSet

	// Granted capabilities by plugin name
	Granted map[string]capability.GrantSet
}

// ExecuteProfileResponse contains the result of profile execution.
type ExecuteProfileResponse struct {
	// ExecutionResult is the domain execution result
	ExecutionResult *execution.ExecutionResult
}

// PlanProfileResponse contains the execution plan for a profile.
type PlanProfileResponse struct {
	// Plan is the dry-run execution plan
	Plan *entities.ExecutionPlan

	// Metadata contains response metadata
	Metadata ResponseMetadata
}

// ValidateProfileResponse contains the result of validating a profile.
type ValidateProfileResponse struct {
	ProfileName string
	Version     string
	Metadata    ResponseMetadata
	Errors      []ValidationError
	Warnings    []string
	Stats       ValidationStats
	Valid       bool
}

// ValidationError represents a single validation failure.
type ValidationError struct {
	Type    string // "structural", "schema", "dependency", "expect"
	Path    string // e.g., "controls[0].observations[1].expect[2]"
	Message string // human-readable error description
}

// ValidationStats provides summary information about the validated profile.
type ValidationStats struct {
	PluginsUsed      []string
	ControlCount     int
	ObservationCount int
	ExpectCount      int
}
