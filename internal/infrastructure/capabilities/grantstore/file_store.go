// OWNERSHIP: REGLET RUNTIME (should NOT be in SDK)
// STATUS: Needs migration to reglet/internal/infrastructure/capabilities/grantstore/

package grantstore

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/reglet-dev/reglet-abi/hostfunc"
	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/domain/capability"
	"gopkg.in/yaml.v3"
)

// fileStoreConfig holds configuration for the FileStore.
type fileStoreConfig struct {
	path     string      // Path to the grants file
	dirPerm  os.FileMode // Permission for created directories
	filePerm os.FileMode // Permission for the grants file
}

func defaultFileStoreConfig() fileStoreConfig {
	return fileStoreConfig{
		path:     filepath.Join(os.Getenv("HOME"), ".reglet", "grants.yaml"),
		dirPerm:  0o755, // User config directory
		filePerm: 0o600, // User-only read/write (secure default)
	}
}

// FileStoreOption configures a FileStore instance.
type FileStoreOption func(*fileStoreConfig)

// WithPath sets the path to the grants file.
func WithPath(path string) FileStoreOption {
	return func(c *fileStoreConfig) {
		c.path = path
	}
}

// WithFilePermissions sets the file permissions for the grants file.
// Default is 0o600 (user-only). Use with caution.
func WithFilePermissions(perm os.FileMode) FileStoreOption {
	return func(c *fileStoreConfig) {
		c.filePerm = perm
	}
}

// WithDirPermissions sets the directory permissions for the grants directory.
// Default is 0o755.
func WithDirPermissions(perm os.FileMode) FileStoreOption {
	return func(c *fileStoreConfig) {
		c.dirPerm = perm
	}
}

// FileStore provides file-based persistence for capability grants.
type FileStore struct {
	config fileStoreConfig
}

// NewFileStore creates a new FileStore with the given options.
func NewFileStore(opts ...FileStoreOption) ports.GrantStore {
	cfg := defaultFileStoreConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return &FileStore{config: cfg}
}

// Load retrieves all granted capabilities.
func (s *FileStore) Load() (capability.GrantSet, error) {
	data, err := os.ReadFile(s.config.path)
	if os.IsNotExist(err) {
		// Return empty set if file doesn't exist
		return capability.GrantSet{}, nil
	}
	if err != nil {
		return capability.GrantSet{}, fmt.Errorf("failed to read grant store: %w", err)
	}

	var grants hostfunc.GrantSet
	if err := yaml.Unmarshal(data, &grants); err != nil {
		return capability.GrantSet{}, fmt.Errorf("failed to parse grant store: %w", err)
	}
	return capability.FromABI(&grants), nil
}

// Save persists the granted capabilities.
func (s *FileStore) Save(grants capability.GrantSet) error {
	// Convert to ABI type for serialization
	abiGrants := capability.ToABI(grants)

	// Clone and deduplicate grants before saving to ensure the config file
	// never contains duplicates, even if they accumulated in memory
	clean := abiGrants.Clone()
	clean.Deduplicate()

	data, err := yaml.Marshal(clean)
	if err != nil {
		return fmt.Errorf("failed to marshal grants: %w", err)
	}

	dir := filepath.Dir(s.config.path)
	if err := os.MkdirAll(dir, s.config.dirPerm); err != nil {
		return fmt.Errorf("failed to create grant store directory: %w", err)
	}

	if err := os.WriteFile(s.config.path, data, s.config.filePerm); err != nil {
		return fmt.Errorf("failed to write grant store: %w", err)
	}
	return nil
}

// ConfigPath returns the path to the backing store.
func (s *FileStore) ConfigPath() string {
	return s.config.path
}
