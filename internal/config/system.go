package config

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/whiskeyjimbo/reglet/internal/wasm/hostfuncs"
)

// SystemConfig represents the global configuration file (~/.reglet/config.yaml).
type SystemConfig struct {
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

type HashModeConfig struct {
	Enabled bool   `yaml:"enabled"`
	Salt    string `yaml:"salt"` // Optional salt for stable hashing
}

// LoadSystemConfig loads the system configuration from the specified path.
// If the file does not exist, it returns an empty config without error.
func LoadSystemConfig(path string) (*SystemConfig, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &SystemConfig{}, nil
	}

	// Read config file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read system config: %w", err)
	}

	// Parse YAML
	var config SystemConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse system config: %w", err)
	}

	return &config, nil
}

// ToHostFuncsCapabilities converts the config capability format to the internal hostfuncs format.
func (sc *SystemConfig) ToHostFuncsCapabilities() []hostfuncs.Capability {
	caps := make([]hostfuncs.Capability, 0, len(sc.Capabilities))
	for _, c := range sc.Capabilities {
		caps = append(caps, hostfuncs.Capability{
			Kind:    c.Kind,
			Pattern: c.Pattern,
		})
	}
	return caps
}
