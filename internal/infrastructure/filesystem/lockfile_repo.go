// Package filesystem provides file-based repositories for the infrastructure layer.
package filesystem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/reglet-dev/reglet/internal/domain/entities"
)

// FileLockfileRepository implements ports.LockfileRepository using the local filesystem.
type FileLockfileRepository struct{}

// NewFileLockfileRepository creates a new FileLockfileRepository.
func NewFileLockfileRepository() *FileLockfileRepository {
	return &FileLockfileRepository{}
}

// Load reads a lockfile from the given path.
func (r *FileLockfileRepository) Load(ctx context.Context, path string) (*entities.Lockfile, error) {
	// Security: Use os.OpenRoot to prevent path traversal attacks if possible
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	// Since we might be running where OpenRoot is preferred but standard Open is common
	// We'll follow the pattern from ProfileLoader
	root, err := os.OpenRoot(dir)
	if err != nil {
		// If directory doesn't exist, lockfile doesn't exist
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open directory %q: %w", dir, err)
	}
	defer func() { _ = root.Close() }()

	file, err := root.Open(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open lockfile %q: %w", base, err)
	}
	defer func() { _ = file.Close() }()

	var lock entities.Lockfile
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&lock); err != nil {
		return nil, fmt.Errorf("decoding lockfile YAML: %w", err)
	}

	// Validate loaded lockfile
	if err := lock.Validate(); err != nil {
		return nil, fmt.Errorf("invalid lockfile: %w", err)
	}

	return &lock, nil
}

// Save writes a lockfile to the given path.
func (r *FileLockfileRepository) Save(ctx context.Context, lockfile *entities.Lockfile, path string) error {
	dir := filepath.Dir(path)

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating directory %q: %w", dir, err)
	}

	// We use standard os.OpenFile for writing as OpenRoot implies read-only usually or directory access.
	// For writing, atomic write is preferred (write temp + rename), but simple write is okay for phase 2.5
	// actually standard library doesn't easily do OpenRoot for writing in a "root constrained" way easily for generic paths?
	// os.Root has Create/OpenFile.

	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("opening directory for write %q: %w", dir, err)
	}
	defer func() { _ = root.Close() }()

	base := filepath.Base(path)

	// Create/Truncate file
	file, err := root.OpenFile(base, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("creating lockfile %q: %w", base, err)
	}
	defer func() { _ = file.Close() }()

	encoder := yaml.NewEncoder(file)
	defer func() { _ = encoder.Close() }()

	if err := encoder.Encode(lockfile); err != nil {
		return fmt.Errorf("encoding lockfile: %w", err)
	}

	return nil
}

// Exists checks if a lockfile exists at the given path.
func (r *FileLockfileRepository) Exists(ctx context.Context, path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
