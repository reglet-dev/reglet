package ports

import (
	"context"
	"time"

	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/values"
	"github.com/reglet-dev/reglet/internal/infrastructure/system"
)

// ProfileLoader loads profiles from storage.
type ProfileLoader interface {
	LoadProfile(path string) (*entities.Profile, error)
	// LoadProfileWithCLIVars loads a profile and merges CLI variables before substitution.
	// CLI variables override profile variables at the same path.
	LoadProfileWithCLIVars(path string, cliVars map[string]interface{}) (*entities.Profile, error)
	// LoadProfileWithOptions loads a profile with CLI variables and remote fetch options.
	// remoteOpts configures behavior for remote profile fetching (refresh, timeout, etc.).
	LoadProfileWithOptions(path string, cliVars map[string]interface{}, remoteOpts RemoteLoadOptions) (*entities.Profile, error)
}

// RemoteLoadOptions configures remote profile loading behavior.
type RemoteLoadOptions struct {
	// Refresh forces cache bypass for remote profiles.
	Refresh bool
	// AllowPrivateNetwork permits fetching from private IPs.
	AllowPrivateNetwork bool
	// Insecure skips TLS validation for remote profiles.
	Insecure bool
	// Timeout for remote fetch operations.
	Timeout time.Duration
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

// ProfileCacheRepository manages persistent storage of cached remote profiles.
// Implements Repository pattern for ProfileCacheEntry aggregate.
type ProfileCacheRepository interface {
	// Find retrieves a cached profile by reference.
	// Returns nil if not found.
	Find(ctx context.Context, ref values.ProfileReference) (*entities.ProfileCacheEntry, error)

	// Store persists a profile cache entry.
	Store(ctx context.Context, entry *entities.ProfileCacheEntry) error

	// List returns all cached profiles.
	List(ctx context.Context) ([]*entities.ProfileCacheEntry, error)

	// Delete removes a specific profile from cache.
	Delete(ctx context.Context, ref values.ProfileReference) error

	// Prune removes profiles older than the specified duration.
	// Returns the number of entries removed.
	Prune(ctx context.Context, maxAge time.Duration) (int, error)
}
