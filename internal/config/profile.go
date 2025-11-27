// Package config provides profile configuration loading and validation for Reglet.
// It handles YAML parsing, variable substitution, and profile validation.
package config

import "time"

// Profile represents a complete Reglet profile configuration.
// Profiles define compliance checks with controls and observations.
type Profile struct {
	Metadata ProfileMetadata        `yaml:"profile"`
	Vars     map[string]interface{} `yaml:"vars,omitempty"`
	Controls ControlsSection        `yaml:"controls"`
}

// ProfileMetadata contains metadata about the profile.
type ProfileMetadata struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Description string `yaml:"description,omitempty"`
}

// ControlsSection contains control defaults and individual controls.
type ControlsSection struct {
	Defaults *ControlDefaults `yaml:"defaults,omitempty"`
	Items    []Control        `yaml:"items"`
}

// ControlDefaults defines default values applied to all controls.
// Individual controls can override these defaults.
type ControlDefaults struct {
	Severity string        `yaml:"severity,omitempty"`
	Owner    string        `yaml:"owner,omitempty"`
	Tags     []string      `yaml:"tags,omitempty"`
	Timeout  time.Duration `yaml:"timeout,omitempty"`
}

// Control represents a single compliance or infrastructure control to validate.
type Control struct {
	ID           string        `yaml:"id"`
	Name         string        `yaml:"name"`
	Description  string        `yaml:"description,omitempty"`
	Severity     string        `yaml:"severity,omitempty"`
	Owner        string        `yaml:"owner,omitempty"`
	Tags         []string      `yaml:"tags,omitempty"`
	Timeout      time.Duration `yaml:"timeout,omitempty"`
	Observations []Observation `yaml:"observations"`
	DependsOn    []string      `yaml:"depends_on,omitempty"` // Control IDs this control depends on
}

// Observation represents a single check to execute using a plugin.
type Observation struct {
	Plugin string                 `yaml:"plugin"`
	Config map[string]interface{} `yaml:"config"`
}
