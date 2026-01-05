// Package ports defines interfaces for infrastructure dependencies.
// These are the "ports" in hexagonal architecture - abstractions that
// the application layer depends on but doesn't implement.
package ports

import (
	"context"

	"github.com/whiskeyjimbo/reglet/internal/application/dto"
	"github.com/whiskeyjimbo/reglet/internal/domain/capabilities"
	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
	"github.com/whiskeyjimbo/reglet/internal/domain/execution"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/system"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/wasm"
)

// ProfileLoader loads profiles from storage.
type ProfileLoader interface {
	LoadProfile(path string) (*entities.Profile, error)
}

// ProfileValidator validates profile structure and schemas.
type ProfileValidator interface {
	Validate(profile *entities.Profile) error
	ValidateWithSchemas(ctx context.Context, profile *entities.Profile, runtime *wasm.Runtime) error
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
	CollectRequiredCapabilities(ctx context.Context, profile entities.ProfileReader, runtime *wasm.Runtime, pluginDir string) (map[string][]capabilities.Capability, error)
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
