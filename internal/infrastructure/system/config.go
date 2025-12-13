// Package system provides infrastructure for system-level configuration.
// This includes loading system config files (~/.reglet/config.yaml) and
// capability grants.
package system

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/whiskeyjimbo/reglet/internal/wasm/hostfuncs"
)

// Config represents the global configuration file (~/.reglet/config.yaml).
// This is infrastructure-level configuration separate from profile configuration.
type Config struct {
	// Plugin capability grants
	Capabilities []struct {
		Kind    string `yaml:"kind"`
		Pattern string `yaml:"pattern"`
	} `yaml:"capabilities"`

	// Redaction configuration for secrets
	Redaction RedactionConfig `yaml:"redaction"`
}

// RedactionConfig configures how sensitive data is sanitized.
type RedactionConfig struct {
	// Custom patterns to redact (regex strings)
	Patterns []string `yaml:"patterns"`
	// JSON paths to always redact (e.g. "config.password")
	Paths []string `yaml:"paths"`
	// If true, replace with hash instead of [REDACTED]
	HashMode HashModeConfig `yaml:"hash_mode"`
}

// HashModeConfig controls hash-based redaction.
type HashModeConfig struct {
	Enabled bool   `yaml:"enabled"`
	Salt    string `yaml:"salt"` // Optional salt for stable hashing
}

// ConfigLoader loads system configuration from disk.
type ConfigLoader struct{}

// NewConfigLoader creates a new system config loader.
func NewConfigLoader() *ConfigLoader {
	return &ConfigLoader{}
}

// Load loads the system configuration from the specified path.
// If the file does not exist, it returns an empty config without error.
func (l *ConfigLoader) Load(path string) (*Config, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &Config{}, nil
	}

	// Read config file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read system config: %w", err)
	}

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse system config: %w", err)
	}

	return &config, nil
}

// ToHostFuncsCapabilities converts the config capability format to the internal hostfuncs format.
func (c *Config) ToHostFuncsCapabilities() []hostfuncs.Capability {
	caps := make([]hostfuncs.Capability, 0, len(c.Capabilities))
	for _, cap := range c.Capabilities {
		caps = append(caps, hostfuncs.Capability{
			Kind:    cap.Kind,
			Pattern: cap.Pattern,
		})
	}
	return caps
}
