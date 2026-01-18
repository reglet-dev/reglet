// Package dto contains data transfer objects for application layer use cases.
package dto

import (
	"github.com/reglet-dev/reglet/internal/domain/capabilities"
)

// CheckProfileRequest encapsulates all inputs needed to check a profile.
type CheckProfileRequest struct {
	Options      CheckOptions
	ProfilePath  string
	Metadata     RequestMetadata
	Filters      FilterOptions
	Execution    ExecutionOptions
	CLIVariables map[string]interface{} // Parsed --set, --set-file, --set-env values
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
	GrantedCapabilities  map[string][]capabilities.Capability
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
