package ports

import (
	"context"
	"io"

	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/values"
)

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
