package config

import "github.com/whiskeyjimbo/reglet/internal/wasm/hostfuncs"

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
	Enabled bool `yaml:"enabled"`
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
