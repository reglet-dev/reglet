// Package system provides infrastructure for system-level configuration.
// This includes loading system config files (~/.reglet/config.yaml) and
// capability grants.
package system

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/reglet-dev/reglet/internal/domain/capabilities"
)

// Config represents the global configuration file (~/.reglet/config.yaml).
// This is infrastructure-level configuration separate from profile configuration.
type Config struct {
	SensitiveData SensitiveDataConfig `yaml:"sensitive_data"`
	Redaction     RedactionConfig     `yaml:"redaction"`
	Security      SecurityConfig      `yaml:"security"`
	Capabilities  []struct {
		Kind    string `yaml:"kind"`
		Pattern string `yaml:"pattern"`
	} `yaml:"capabilities"`
	WasmMemoryLimitMB    int `yaml:"wasm_memory_limit_mb"`
	MaxEvidenceSizeBytes int `yaml:"max_evidence_size_bytes"`
}

// SensitiveDataConfig configures secret resolution and protection.
// This structure is forward-compatible with future phases (OIDC, Cloud).
type SensitiveDataConfig struct {
	Secrets SecretsConfig `yaml:"secrets"`
}

// SecretsConfig configures secret resolution sources.
type SecretsConfig struct {
	// Local defines static secrets for development (name -> value)
	Local map[string]string `yaml:"local"`

	// Env defines environment variable mappings (secret_name -> env_var_name)
	Env map[string]string `yaml:"env"`

	// Files defines file path mappings (secret_name -> file_path)
	Files map[string]string `yaml:"files"`
}

// RedactionConfig configures how sensitive data is sanitized.
type RedactionConfig struct {
	HashMode HashModeConfig `yaml:"hash_mode"`
	Patterns []string       `yaml:"patterns"`
	Paths    []string       `yaml:"paths"`
}

// HashModeConfig controls hash-based redaction.
type HashModeConfig struct {
	Salt    string `yaml:"salt"`
	Enabled bool   `yaml:"enabled"`
}

// SecurityConfig configures capability security policies.
type SecurityConfig struct {
	// Level defines the security policy: "strict", "standard", or "permissive"
	// - strict: Deny all broad capabilities
	// - standard: Warn about broad capabilities (default)
	// - permissive: Allow all capabilities without warnings
	Level string `yaml:"level"`

	// CustomBroadPatterns allows users to define additional patterns considered "broad"
	// Format: "kind:pattern" (e.g., "fs:write:/tmp/**")
	CustomBroadPatterns []string `yaml:"custom_broad_patterns"`
}

// SecurityLevel represents the security enforcement level.
type SecurityLevel string

const (
	// SecurityLevelStrict denies broad capabilities
	SecurityLevelStrict SecurityLevel = "strict"

	// SecurityLevelStandard warns about broad capabilities (default)
	SecurityLevelStandard SecurityLevel = "standard"

	// SecurityLevelPermissive allows all capabilities without warnings
	SecurityLevelPermissive SecurityLevel = "permissive"
)

// GetSecurityLevel returns the configured security level, defaulting to Standard.
func (c *SecurityConfig) GetSecurityLevel() SecurityLevel {
	switch c.Level {
	case "strict":
		return SecurityLevelStrict
	case "standard":
		return SecurityLevelStandard
	case "permissive":
		return SecurityLevelPermissive
	default:
		// Default to standard if not specified or invalid
		return SecurityLevelStandard
	}
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
	//nolint:gosec // G304: path is user-provided config file, validated to exist above
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
func (c *Config) ToHostFuncsCapabilities() []capabilities.Capability {
	caps := make([]capabilities.Capability, 0, len(c.Capabilities))
	for _, capability := range c.Capabilities {
		caps = append(caps, capabilities.Capability{
			Kind:    capability.Kind,
			Pattern: capability.Pattern,
		})
	}
	return caps
}
