package dto

import (
	"time"

	"github.com/reglet-dev/reglet/internal/domain/capability"
)

// CheckProfileRequest encapsulates all inputs needed to check a profile.
type CheckProfileRequest struct {
	CLIVariables  map[string]interface{}
	ProfilePath   string
	Metadata      RequestMetadata
	Options       CheckOptions
	Filters       FilterOptions
	Execution     ExecutionOptions
	RemoteOptions RemoteProfileOptions
}

// RemoteProfileOptions configures remote profile fetching behavior.
type RemoteProfileOptions struct {
	// Timeout overrides the default fetch timeout.
	Timeout time.Duration

	// Refresh forces a cache bypass and re-fetch.
	Refresh bool

	// AllowPrivateNetwork permits fetching from private IP addresses.
	AllowPrivateNetwork bool

	// Insecure skips TLS certificate validation (not recommended).
	Insecure bool

	// TrustSource bypasses interactive trust prompt for remote profiles.
	TrustSource bool
}

// FilterOptions defines filters for control selection.
type FilterOptions struct {
	FilterExpression    string
	IncludeTags         []string
	IncludeSeverities   []string
	IncludeControlIDs   []string
	ExcludeTags         []string
	ExcludeControlIDs   []string
	IncludeDependencies bool
}

// ExecutionOptions controls how the profile is executed.
type ExecutionOptions struct {
	// Parallel enables parallel execution of controls
	Parallel bool

	// MaxConcurrentControls limits parallel control execution (0 = no limit)
	MaxConcurrentControls int

	// MaxConcurrentObservations limits parallel observation execution (0 = no limit)
	MaxConcurrentObservations int
}

// CheckOptions contains options for plugin and capability management.
type CheckOptions struct {
	PluginDir            string
	SystemConfigPath     string
	TrustPlugins         bool
	SkipSchemaValidation bool
	WarnUnusedVars       bool // Enable warnings for unused CLI variables (enabled by default when CLI vars are set)
}

// RequestMetadata contains metadata for request tracking.
type RequestMetadata struct {
	// RequestID uniquely identifies this request
	RequestID string
}

// LoadProfileRequest encapsulates inputs for loading a profile.
type LoadProfileRequest struct {
	ProfilePath string
}

// CollectCapabilitiesRequest encapsulates inputs for capability collection.
type CollectCapabilitiesRequest struct {
	ProfilePath  string
	PluginDir    string
	TrustPlugins bool
}

// ExecuteProfileRequest encapsulates inputs for profile execution.
type ExecuteProfileRequest struct {
	GrantedCapabilities  map[string]capability.GrantSet
	ProfilePath          string
	Filters              FilterOptions
	Execution            ExecutionOptions
	SkipSchemaValidation bool
}

// PlanProfileRequest encapsulates inputs for planning profile execution.
// It generates a dry-run execution plan without actually running controls.
type PlanProfileRequest struct {
	ProfilePath string
	Metadata    RequestMetadata
	Filters     FilterOptions
}

// ValidateProfileRequest encapsulates inputs for validating a profile.
// It validates structure and syntax without execution.
type ValidateProfileRequest struct {
	ProfilePath          string
	Metadata             RequestMetadata
	SkipSchemaValidation bool
	SkipExpectValidation bool
}
