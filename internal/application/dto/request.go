// Package dto contains data transfer objects for application layer use cases.
package dto

import (
	"github.com/reglet-dev/reglet/internal/domain/capabilities"
)

// CheckProfileRequest encapsulates all inputs needed to check a profile.
type CheckProfileRequest struct {
	// ProfilePath is the path to the profile YAML file
	ProfilePath string

	// Filters control which controls to execute
	Filters FilterOptions

	// Execution controls how the profile is executed
	Execution ExecutionOptions

	// Options for plugin and capability management
	Options CheckOptions

	// Metadata for request tracking and diagnostics
	Metadata RequestMetadata
}

// FilterOptions defines filters for control selection.
type FilterOptions struct {
	// IncludeTags - only run controls with these tags (OR logic)
	IncludeTags []string

	// IncludeSeverities - only run controls with these severities (OR logic)
	IncludeSeverities []string

	// IncludeControlIDs - only run these specific controls (exclusive)
	IncludeControlIDs []string

	// ExcludeTags - skip controls with these tags
	ExcludeTags []string

	// ExcludeControlIDs - skip these specific controls
	ExcludeControlIDs []string

	// FilterExpression - advanced filter using expr language
	FilterExpression string

	// IncludeDependencies - automatically include dependency controls
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
	// TrustPlugins - if true, auto-grant all capabilities
	TrustPlugins bool

	// PluginDir - custom plugin directory (empty = auto-detect)
	PluginDir string

	// SkipSchemaValidation - skip plugin config schema validation
	SkipSchemaValidation bool

	// SystemConfigPath - custom system config path (empty = default ~/.reglet/config.yaml)
	SystemConfigPath string
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
	ProfilePath          string
	Filters              FilterOptions
	Execution            ExecutionOptions
	GrantedCapabilities  map[string][]capabilities.Capability
	SkipSchemaValidation bool
}
