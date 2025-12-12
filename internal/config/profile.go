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

// HasAnyTag checks if the control has any of the specified tags.
func (c *Control) HasAnyTag(tags []string) bool {
	if len(tags) == 0 {
		return false
	}
	for _, t := range tags {
		if c.HasTag(t) {
			return true
		}
	}
	return false
}

// MatchesAnySeverity checks if the control matches any of the specified severities.
func (c *Control) MatchesAnySeverity(severities []string) bool {
	if len(severities) == 0 {
		return false
	}
	mySev, err := valueobjects.NewSeverity(c.Severity)
	if err != nil {
		return false
	}
	for _, s := range severities {
		target, err := valueobjects.NewSeverity(s)
		if err == nil && mySev.Equals(target) {
			return true
		}
	}
	return false
}

// HasDependency checks if the control depends on the specified control ID.
func (c *Control) HasDependency(controlID string) bool {
	for _, dep := range c.DependsOn {
		if dep == controlID {
			return true
		}
	}
	return false
}

// GetEffectiveTimeout returns the control's timeout or the default if not set.
func (c *Control) GetEffectiveTimeout(defaultTimeout time.Duration) time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return defaultTimeout
}

// ObservationCount returns the number of observations in the control.
func (c *Control) ObservationCount() int {
	return len(c.Observations)
}

// IsEmpty returns true if the control is empty (no ID, name, or observations).
func (c *Control) IsEmpty() bool {
	return c.ID == "" && c.Name == "" && len(c.Observations) == 0
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

// Validate checks the profile structure and content.
func (p *Profile) Validate() error {
	if p.Metadata.Name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}
	if len(p.Controls.Items) == 0 {
		return fmt.Errorf("profile must have at least one control")
	}
	for _, ctrl := range p.Controls.Items {
		if err := ctrl.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// GetControl returns a pointer to the control with the given ID, or nil if not found.
func (p *Profile) GetControl(id string) *Control {
	for i := range p.Controls.Items {
		if p.Controls.Items[i].ID == id {
			return &p.Controls.Items[i]
		}
	}
	return nil
}

// HasControl checks if a control with the given ID exists in the profile.
func (p *Profile) HasControl(id string) bool {
	return p.GetControl(id) != nil
}

// ControlCount returns the number of controls in the profile.
func (p *Profile) ControlCount() int {
	return len(p.Controls.Items)
}

// SelectControlsByTags returns controls that match any of the specified tags.
func (p *Profile) SelectControlsByTags(tags []string) []Control {
	var selected []Control
	for _, ctrl := range p.Controls.Items {
		if ctrl.HasAnyTag(tags) {
			selected = append(selected, ctrl)
		}
	}
	return selected
}

// SelectControlsBySeverity returns controls that match any of the specified severities.
func (p *Profile) SelectControlsBySeverity(severities []string) []Control {
	var selected []Control
	for _, ctrl := range p.Controls.Items {
		if ctrl.MatchesAnySeverity(severities) {
			selected = append(selected, ctrl)
		}
	}
	return selected
}

// ExcludeControlsByID returns controls except those with the specified IDs.
func (p *Profile) ExcludeControlsByID(excludeIDs []string) []Control {
	excludeMap := make(map[string]bool)
	for _, id := range excludeIDs {
		excludeMap[id] = true
	}
	var selected []Control
	for _, ctrl := range p.Controls.Items {
		if !excludeMap[ctrl.ID] {
			selected = append(selected, ctrl)
		}
	}
	return selected
}

// ApplyDefaults applies control defaults to all controls in the profile.
func (p *Profile) ApplyDefaults() {
	if p.Controls.Defaults == nil {
		return
	}

	defaults := p.Controls.Defaults

	for i := range p.Controls.Items {
		ctrl := &p.Controls.Items[i]

		if ctrl.Severity == "" && defaults.Severity != "" {
			ctrl.Severity = defaults.Severity
		}
		if ctrl.Owner == "" && defaults.Owner != "" {
			ctrl.Owner = defaults.Owner
		}
		if ctrl.Timeout == 0 && defaults.Timeout != 0 {
			ctrl.Timeout = defaults.Timeout
		}

		if len(defaults.Tags) > 0 {
			tagMap := make(map[string]bool)
			for _, tag := range defaults.Tags {
				tagMap[tag] = true
			}
			for _, tag := range ctrl.Tags {
				tagMap[tag] = true
			}
			mergedTags := make([]string, 0, len(tagMap))
			for tag := range tagMap {
				mergedTags = append(mergedTags, tag)
			}
			ctrl.Tags = mergedTags
		}
	}
}
