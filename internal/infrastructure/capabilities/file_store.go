// Package capabilities provides capabilities for the wasm plugins
package capabilities

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/reglet-dev/reglet/internal/domain/capabilities"
)

// FileStore provides file-based persistence for capability grants.
type FileStore struct {
	configPath string
}

// NewFileStore creates a new FileStore.
func NewFileStore(configPath string) *FileStore {
	return &FileStore{
		configPath: configPath,
	}
}

// ConfigPath returns the path to the config file.
func (s *FileStore) ConfigPath() string {
	return s.configPath
}

// configFile represents the YAML structure of ~/.reglet/config.yaml
type configFile struct {
	Capabilities []struct {
		Kind    string `yaml:"kind"`
		Pattern string `yaml:"pattern"`
	} `yaml:"capabilities"`
}

// Load loads capability grants from ~/.reglet/config.yaml.
// If the file does not exist, it returns an empty Grant without error.
func (s *FileStore) Load() (capabilities.Grant, error) {
	// Check if config file exists
	if _, err := os.Stat(s.configPath); os.IsNotExist(err) {
		return capabilities.NewGrant(), nil
	}

	// Read config file
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var cfg configFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Convert to capability slice
	caps := capabilities.NewGrant()
	for _, c := range cfg.Capabilities {
		caps.Add(capabilities.Capability{
			Kind:    c.Kind,
			Pattern: c.Pattern,
		})
	}

	return caps, nil
}

// Save saves capability grants to ~/.reglet/config.yaml.
func (s *FileStore) Save(grants capabilities.Grant) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(s.configPath)
	//nolint:gosec // G301: 0o755 is standard for user config directories (~/.reglet)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Convert domain Grant to configFile struct
	cfgCaps := make([]struct {
		Kind    string `yaml:"kind"`
		Pattern string `yaml:"pattern"`
	}, len(grants))

	for i, capability := range grants {
		cfgCaps[i].Kind = capability.Kind
		cfgCaps[i].Pattern = capability.Pattern
	}

	cfg := configFile{Capabilities: cfgCaps}

	// Marshal to YAML
	data, err := yaml.MarshalWithOptions(cfg, yaml.IndentSequence(true))
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	return os.WriteFile(s.configPath, data, 0o600)
}
