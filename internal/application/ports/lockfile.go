package ports

import (
	"context"

	"github.com/reglet-dev/reglet/internal/domain/entities"
)

type VersionResolver interface {
	// Resolve converts a version constraint to an exact version.
	// Examples:
	//   "@1.0"   -> "1.0.x" (latest 1.0.x)
	//   "^1.2.0" -> "1.x.x >= 1.2.0"
	//   "~1.2.3" -> "1.2.x >= 1.2.3"
	Resolve(constraint string, available []string) (string, error)
}

type LockfileRepository interface {
	// Load reads a lockfile from the given path.
	// Returns nil, nil if lockfile doesn't exist.
	Load(ctx context.Context, path string) (*entities.Lockfile, error)

	// Save writes a lockfile to the given path.
	Save(ctx context.Context, lockfile *entities.Lockfile, path string) error

	// Exists checks if a lockfile exists at the given path.
	Exists(ctx context.Context, path string) (bool, error)
}

// PluginDigester computes digests for plugins.
type PluginDigester interface {
	// DigestBytes computes SHA-256 of raw bytes.
	DigestBytes(data []byte) string

	// DigestFile computes SHA-256 of a file.
	DigestFile(ctx context.Context, path string) (string, error)
}
