// Package entities contains domain entities for the Reglet domain model.
// These are pure domain types with NO infrastructure dependencies.
package entities

import (
	"fmt"
	"time"
)

// Profile represents the Reglet profile configuration.
// Aggregate root in the Configuration context.
//
// Profile defines the validation configuration and ruleset.
// It serves as the aggregate root for the configuration context.
//
// Invariants enforced:
// - Unique control IDs
// - All dependencies must exist
// - Name and version are mandatory
// - At least one observation per control
type Profile struct {
	Metadata ProfileMetadata        `yaml:"profile"`
	Plugins  []string               `yaml:"plugins,omitempty"`
	Vars     map[string]interface{} `yaml:"vars,omitempty"`
	Controls ControlsSection        `yaml:"controls"`
}

// ProfileMetadata contains descriptive information about the profile.
type ProfileMetadata struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Description string `yaml:"description,omitempty"`
}

// ControlsSection groups validation controls and their default settings.
type ControlsSection struct {
	Defaults *ControlDefaults `yaml:"defaults,omitempty"`
	Items    []Control        `yaml:"items"`
}

// ControlDefaults specifies values inherited by controls when not explicitly set.
type ControlDefaults struct {
	Severity string        `yaml:"severity,omitempty"`
	Owner    string        `yaml:"owner,omitempty"`
	Tags     []string      `yaml:"tags,omitempty"`
	Timeout  time.Duration `yaml:"timeout,omitempty"`
}

// Control represents a specific compliance check or validation unit.
// It is uniquely identified by its ID.
type Control struct {
	ID           string        `yaml:"id"`
	Name         string        `yaml:"name"`
	Description  string        `yaml:"description,omitempty"`
	Severity     string        `yaml:"severity,omitempty"`
	Owner        string        `yaml:"owner,omitempty"`
	Tags         []string      `yaml:"tags,omitempty"`
		DependsOn              []string                `yaml:"depends_on,omitempty"`
		Timeout                time.Duration           `yaml:"timeout,omitempty"`
		ObservationDefinitions []ObservationDefinition `yaml:"observations"`
	}

// ObservationDefinition configuration for a specific plugin execution.
// It is an immutable value object.
// Renamed from Observation to ObservationDefinition to avoid confusion with ObservationResult.
type ObservationDefinition struct {
	Plugin string                 `yaml:"plugin"`
	Config map[string]interface{} `yaml:"config,omitempty"`
	Expect []string               `yaml:"expect,omitempty"`
}

// ===== PROFILE AGGREGATE ROOT METHODS =====

// Validate checks the integrity of the profile configuration.
func (p *Profile) Validate() error {
	if p.Metadata.Name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}
	if p.Metadata.Version == "" {
		return fmt.Errorf("profile version cannot be empty")
	}

	if len(p.Controls.Items) == 0 {
		return fmt.Errorf("at least one control is required")
	}

	controlIDs := make(map[string]bool)
	for i, ctrl := range p.Controls.Items {
		if err := ctrl.Validate(); err != nil {
			return fmt.Errorf("control %d (%s): %w", i, ctrl.ID, err)
		}

		if controlIDs[ctrl.ID] {
			return fmt.Errorf("duplicate control ID: %s", ctrl.ID)
		}
		controlIDs[ctrl.ID] = true
	}

	for _, ctrl := range p.Controls.Items {
		for _, dep := range ctrl.DependsOn {
			if !controlIDs[dep] {
				return fmt.Errorf("control %s depends on non-existent control %s", ctrl.ID, dep)
			}
		}
	}

	return nil
}

// AddControl safely adds a new control to the profile.
// It returns an error if the control is invalid or already exists.
func (p *Profile) AddControl(ctrl Control) error {
	if err := ctrl.Validate(); err != nil {
		return fmt.Errorf("invalid control: %w", err)
	}

	for _, existing := range p.Controls.Items {
		if existing.ID == ctrl.ID {
			return fmt.Errorf("control with ID %s already exists", ctrl.ID)
		}
	}

	for _, dep := range ctrl.DependsOn {
		found := false
		for _, existing := range p.Controls.Items {
			if existing.ID == dep {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("control %s depends on non-existent control %s", ctrl.ID, dep)
		}
	}

	p.Controls.Items = append(p.Controls.Items, ctrl)
	return nil
}

// GetControl retrieves a control by its ID.
// It returns nil if the control is not found.
func (p *Profile) GetControl(id string) *Control {
	for i := range p.Controls.Items {
		if p.Controls.Items[i].ID == id {
			return &p.Controls.Items[i]
		}
	}
	return nil
}

// HasControl reports whether a control with the given ID exists.
func (p *Profile) HasControl(id string) bool {
	return p.GetControl(id) != nil
}

// ControlCount returns the total number of controls.
func (p *Profile) ControlCount() int {
	return len(p.Controls.Items)
}

// SelectControlsByTags returns a subset of controls matching any of the specified tags.
// If tags is empty, all controls are returned.
func (p *Profile) SelectControlsByTags(tags []string) []Control {
	if len(tags) == 0 {
		return p.Controls.Items
	}

	var selected []Control
	for _, ctrl := range p.Controls.Items {
		if ctrl.HasAnyTag(tags) {
			selected = append(selected, ctrl)
		}
	}
	return selected
}

// SelectControlsBySeverity returns a subset of controls matching any of the specified severities.
// If severities is empty, all controls are returned.
func (p *Profile) SelectControlsBySeverity(severities []string) []Control {
	if len(severities) == 0 {
		return p.Controls.Items
	}

	var selected []Control
	for _, ctrl := range p.Controls.Items {
		if ctrl.MatchesAnySeverity(severities) {
			selected = append(selected, ctrl)
		}
	}
	return selected
}

// ExcludeControlsByID returns a subset of controls excluding the specified IDs.
func (p *Profile) ExcludeControlsByID(excludeIDs []string) []Control {
	if len(excludeIDs) == 0 {
		return p.Controls.Items
	}

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

// ApplyDefaults propagates default values to all controls in the profile.
func (p *Profile) ApplyDefaults() {
	if p.Controls.Defaults == nil {
		return
	}

	defaults := p.Controls.Defaults

	for i := range p.Controls.Items {
		ctrl := &p.Controls.Items[i]

		// Apply default severity if not set
		if ctrl.Severity == "" && defaults.Severity != "" {
			ctrl.Severity = defaults.Severity
		}

		// Apply default owner if not set
		if ctrl.Owner == "" && defaults.Owner != "" {
			ctrl.Owner = defaults.Owner
		}

		// Merge tags (defaults + control tags)
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

		// Apply default timeout if not set
		if ctrl.Timeout == 0 && defaults.Timeout > 0 {
			ctrl.Timeout = defaults.Timeout
		}
	}
}

// ===== CONTROL ENTITY METHODS =====

// Validate ensures the control is well-formed.
func (c *Control) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("control ID cannot be empty")
	}
	if c.Name == "" {
		return fmt.Errorf("control %s: name cannot be empty", c.ID)
	}
	if len(c.ObservationDefinitions) == 0 {
		return fmt.Errorf("control %s: must have at least one observation", c.ID)
	}

	// Validate severity if set
	if c.Severity != "" {
		validSeverities := []string{"low", "medium", "high", "critical"}
		valid := false
		for _, sev := range validSeverities {
			if c.Severity == sev {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("control %s: invalid severity %q (must be low, medium, high, or critical)", c.ID, c.Severity)
		}
	}

	return nil
}

// HasTag returns true if the control has the specified tag.
func (c *Control) HasTag(tag string) bool {
	for _, t := range c.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// HasAnyTag returns true if the control has any of the specified tags.
func (c *Control) HasAnyTag(tags []string) bool {
	for _, tag := range tags {
		if c.HasTag(tag) {
			return true
		}
	}
	return false
}

// MatchesSeverity returns true if the control matches the specified severity.
func (c *Control) MatchesSeverity(severity string) bool {
	return c.Severity == severity
}

// MatchesAnySeverity returns true if the control matches any of the severities.
func (c *Control) MatchesAnySeverity(severities []string) bool {
	for _, sev := range severities {
		if c.MatchesSeverity(sev) {
			return true
		}
	}
	return false
}

// HasDependency returns true if the control depends on the specified control ID.
func (c *Control) HasDependency(controlID string) bool {
	for _, dep := range c.DependsOn {
		if dep == controlID {
			return true
		}
	}
	return false
}

// GetEffectiveTimeout returns the control's timeout with fallback to default.
func (c *Control) GetEffectiveTimeout(defaultTimeout time.Duration) time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return defaultTimeout
}

// ObservationCount returns the number of observations in this control.
func (c *Control) ObservationCount() int {
	return len(c.ObservationDefinitions)
}

// IsEmpty returns true if this is the zero value.
func (c *Control) IsEmpty() bool {
	return c.ID == "" && c.Name == ""
}
