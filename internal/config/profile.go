// Package config provides profile configuration loading and validation for Reglet.
// It handles YAML parsing, variable substitution, and profile validation.
package config

import (
	"fmt"
	"time"

	"github.com/whiskeyjimbo/reglet/internal/domain/valueobjects"
)

// Profile represents a complete Reglet profile configuration.
// Profiles define compliance checks with controls and observations.
type Profile struct {
	Metadata ProfileMetadata        `yaml:"profile"`
	Plugins  []string               `yaml:"plugins,omitempty"`
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
	Expect []string               `yaml:"expect,omitempty"`
}

// Validate checks the consistency of the control.
func (c *Control) Validate() error {
	if _, err := valueobjects.NewControlID(c.ID); err != nil {
		return fmt.Errorf("invalid control ID '%s': %w", c.ID, err)
	}
	if c.Name == "" {
		return fmt.Errorf("control name cannot be empty (id: %s)", c.ID)
	}
	if len(c.Observations) == 0 {
		return fmt.Errorf("control must have at least one observation (id: %s)", c.ID)
	}
	if c.Severity != "" {
		if _, err := valueobjects.NewSeverity(c.Severity); err != nil {
			return fmt.Errorf("invalid severity '%s' in control %s: %w", c.Severity, c.ID, err)
		}
	}
	return nil
}

// HasTag checks if the control has a specific tag.
func (c *Control) HasTag(tag string) bool {
	for _, t := range c.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// MatchesSeverity checks if the control meets a minimum severity threshold.
func (c *Control) MatchesSeverity(minSeverity valueobjects.Severity) bool {
	sev, err := valueobjects.NewSeverity(c.Severity)
	if err != nil {
		// Treat invalid/empty severity as lowest possible (Low/Unknown)
		return false
	}
	return sev.IsHigherOrEqual(minSeverity)
}

// AddControl adds a control to the profile with validation.
func (p *Profile) AddControl(ctrl Control) error {
	if err := ctrl.Validate(); err != nil {
		return err
	}
	// Check for duplicates
	for _, existing := range p.Controls.Items {
		if existing.ID == ctrl.ID {
			return fmt.Errorf("duplicate control ID: %s", ctrl.ID)
		}
	}
	p.Controls.Items = append(p.Controls.Items, ctrl)
	return nil
}