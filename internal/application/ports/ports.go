// Package ports defines interfaces for infrastructure dependencies.
// These are the "ports" in hexagonal architecture - abstractions that
// the application layer depends on but doesn't implement.
package ports

import (
	"context"
	"io"

	"github.com/whiskeyjimbo/reglet/internal/application/dto"
	"github.com/whiskeyjimbo/reglet/internal/domain/capabilities"
	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
	"github.com/whiskeyjimbo/reglet/internal/domain/execution"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/system"
)

// PluginInfo contains metadata about a plugin.
// This is the application-layer representation of plugin metadata.
type PluginInfo struct {
	Name         string
	Version      string
	Description  string
	Capabilities []capabilities.Capability
}

// Plugin represents a loaded WASM plugin that can be inspected and executed.
// This interface abstracts the concrete wasm.Plugin implementation.
type Plugin interface {
	// Describe returns plugin metadata including declared capabilities.
	Describe(ctx context.Context) (*PluginInfo, error)
}

// PluginRuntime abstracts the WASM runtime for plugin loading and management.
// This interface allows the application layer to work with plugins without
// depending on concrete infrastructure types like wasm.Runtime.
type PluginRuntime interface {
	// LoadPlugin compiles and caches a plugin from WASM bytes.
	LoadPlugin(ctx context.Context, name string, wasmBytes []byte) (Plugin, error)

	// Close releases runtime resources.
	Close(ctx context.Context) error
}

// PluginRuntimeFactory creates runtime instances.
// This allows the application layer to create runtimes without importing infrastructure.
type PluginRuntimeFactory interface {
	// NewRuntime creates a new plugin runtime for capability collection.
	NewRuntime(ctx context.Context) (PluginRuntime, error)

	// NewRuntimeWithCapabilities creates a runtime with granted capabilities.
	NewRuntimeWithCapabilities(
		ctx context.Context,
		caps map[string][]capabilities.Capability,
		memoryLimitMB int,
	) (PluginRuntime, error)
}

// ProfileLoader loads profiles from storage.
type ProfileLoader interface {
	LoadProfile(path string) (*entities.Profile, error)
}

// ProfileValidator validates profile structure and schemas.
type ProfileValidator interface {
	Validate(profile *entities.Profile) error
	ValidateWithSchemas(ctx context.Context, profile *entities.Profile, runtime PluginRuntime) error
}

// SystemConfigProvider loads system configuration.
type SystemConfigProvider interface {
	LoadConfig(ctx context.Context, path string) (*system.Config, error)
}

// PluginDirectoryResolver resolves the plugin directory path.
type PluginDirectoryResolver interface {
	ResolvePluginDir(ctx context.Context) (string, error)
}

// CapabilityCollector collects required capabilities from plugins.
type CapabilityCollector interface {
	CollectRequiredCapabilities(ctx context.Context, profile entities.ProfileReader, runtime PluginRuntime, pluginDir string) (map[string][]capabilities.Capability, error)
}

// CapabilityAnalyzer extracts specific capability requirements from profiles.
// This allows the orchestrator to be tested with mock analyzers.
type CapabilityAnalyzer interface {
	ExtractCapabilities(profile entities.ProfileReader) map[string][]capabilities.Capability
}

// CapabilityGatekeeperPort grants capabilities based on security policy.
// Named with "Port" suffix to avoid collision with the concrete CapabilityGatekeeper type.
type CapabilityGatekeeperPort interface {
	GrantCapabilities(
		required capabilities.Grant,
		capabilityInfo map[string]CapabilityInfo,
		trustAll bool,
	) (capabilities.Grant, error)
}

// CapabilityGranter grants capabilities (interactively or automatically).
type CapabilityGranter interface {
	GrantCapabilities(ctx context.Context, required map[string][]capabilities.Capability, trustAll bool) (map[string][]capabilities.Capability, error)
}

// ExecutionEngine executes profiles and returns results.
type ExecutionEngine interface {
	Execute(ctx context.Context, profile entities.ProfileReader) (*execution.ExecutionResult, error)
	Close(ctx context.Context) error
}

// EngineFactory creates execution engines with capabilities.
type EngineFactory interface {
	CreateEngine(ctx context.Context, profile entities.ProfileReader, grantedCaps map[string][]capabilities.Capability, pluginDir string, filters dto.FilterOptions, execution dto.ExecutionOptions, skipSchemaValidation bool) (ExecutionEngine, error)
}

// OutputFormatter formats execution results.
type OutputFormatter interface {
	Format(result *execution.ExecutionResult) error
}

// OutputWriter writes formatted output to destination.
type OutputWriter interface {
	Write(ctx context.Context, data []byte, dest string) error
}

// Closer is a common interface for resources that need cleanup.
type Closer interface {
	io.Closer
}
