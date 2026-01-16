// Package entities contains domain entities for the Reglet domain model.
// These are pure domain types with NO infrastructure dependencies.
package entities

import (
	"fmt"
	"time"
)

// BackoffType defines the strategy for retry delays.
type BackoffType string

const (
	BackoffNone        BackoffType = "none"
	BackoffLinear      BackoffType = "linear"
	BackoffExponential BackoffType = "exponential"
)

// ControlDefaults specifies values inherited by controls when not explicitly set.
type ControlDefaults struct {
	Severity      string
	Owner         string
	RetryBackoff  BackoffType
	Tags          []string
	Timeout       time.Duration
	Retries       int
	RetryDelay    time.Duration
	RetryMaxDelay time.Duration
}

// Control represents a specific compliance check or validation unit.
// It is uniquely identified by its ID.
type Control struct {
	ID                     string
	Name                   string
	Description            string
	Severity               string
	Owner                  string
	RetryBackoff           BackoffType
	DependsOn              []string
	ObservationDefinitions []ObservationDefinition
	Tags                   []string
	Timeout                time.Duration
	Retries                int
	RetryDelay             time.Duration
	RetryMaxDelay          time.Duration
}

// LoopConfig defines iteration settings for an observation.
// When specified, the observation will be executed once per item in the list.
type LoopConfig struct {
	Items string // Template expression for list, e.g., "{{ .vars.services }}"
	As    string // Optional variable name (default: uses .loop.item)
}

// ObservationDefinition configuration for a specific plugin execution.
// It is an immutable value object.
type ObservationDefinition struct {
	Plugin string
	Config map[string]interface{}
	Expect []string
	Loop   *LoopConfig // Optional loop configuration
}

// ApplyDefaults applies the given defaults to the control if values are missing.
func (c *Control) ApplyDefaults(defaults *ControlDefaults) {
	if c.Severity == "" && defaults.Severity != "" {
		c.Severity = defaults.Severity
	}

	if c.Owner == "" && defaults.Owner != "" {
		c.Owner = defaults.Owner
	}

	c.applyTagDefaults(defaults.Tags)

	if c.Timeout == 0 && defaults.Timeout > 0 {
		c.Timeout = defaults.Timeout
	}

	c.applyRetryDefaults(defaults)
}

func (c *Control) applyTagDefaults(defaultTags []string) {
	if len(defaultTags) == 0 {
		return
	}

	tagMap := make(map[string]bool)
	for _, tag := range defaultTags {
		tagMap[tag] = true
	}
	for _, tag := range c.Tags {
		tagMap[tag] = true
	}

	mergedTags := make([]string, 0, len(tagMap))
	for tag := range tagMap {
		mergedTags = append(mergedTags, tag)
	}
	c.Tags = mergedTags
}

func (c *Control) applyRetryDefaults(defaults *ControlDefaults) {
	if c.Retries == 0 && defaults.Retries > 0 {
		c.Retries = defaults.Retries
	}

	if c.RetryDelay == 0 && defaults.RetryDelay > 0 {
		c.RetryDelay = defaults.RetryDelay
	}

	if c.RetryBackoff == "" && defaults.RetryBackoff != "" {
		c.RetryBackoff = defaults.RetryBackoff
	}

	if c.RetryMaxDelay == 0 && defaults.RetryMaxDelay > 0 {
		c.RetryMaxDelay = defaults.RetryMaxDelay
	}
}

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
