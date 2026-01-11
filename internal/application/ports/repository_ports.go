package ports

import (
	"context"
	"io"

	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/values"
	"github.com/reglet-dev/reglet/internal/infrastructure/system"
)

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

// PluginRepository manages persistent storage of cached plugins.
// Implements Repository pattern for Plugin aggregate.
type PluginRepository interface {
	// Find retrieves a cached plugin by reference.
	Find(ctx context.Context, ref values.PluginReference) (*entities.Plugin, string, error)

	// Store persists a plugin with its WASM binary.
	// Returns the path to the stored WASM file.
	Store(ctx context.Context, plugin *entities.Plugin, wasm io.Reader) (string, error)

	// List returns all cached plugins.
	List(ctx context.Context) ([]*entities.Plugin, error)

	// Prune removes old versions, keeping only the specified number.
	Prune(ctx context.Context, keepVersions int) error

	// Delete removes a specific plugin from cache.
	Delete(ctx context.Context, ref values.PluginReference) error
}
