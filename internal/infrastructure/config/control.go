package config

import (
	"time"

	"github.com/reglet-dev/reglet/internal/domain/entities"
)

// Controls represents the controls section in YAML.
type Controls struct {
	Defaults *Defaults `yaml:"defaults,omitempty"`
	Items    []Control `yaml:"items"`
}

// Defaults represents the defaults section in YAML.
type Defaults struct {
	Severity      string        `yaml:"severity,omitempty"`
	Owner         string        `yaml:"owner,omitempty"`
	RetryBackoff  string        `yaml:"retry_backoff,omitempty"`
	Tags          []string      `yaml:"tags,omitempty"`
	Timeout       time.Duration `yaml:"timeout,omitempty"`
	Retries       int           `yaml:"retries,omitempty"`
	RetryDelay    time.Duration `yaml:"retry_delay,omitempty"`
	RetryMaxDelay time.Duration `yaml:"retry_max_delay,omitempty"`
}

// Control represents a control in YAML.
type Control struct {
	ID            string        `yaml:"id"`
	Name          string        `yaml:"name"`
	Description   string        `yaml:"description,omitempty"`
	Severity      string        `yaml:"severity,omitempty"`
	Owner         string        `yaml:"owner,omitempty"`
	RetryBackoff  string        `yaml:"retry_backoff,omitempty"`
	DependsOn     []string      `yaml:"depends_on,omitempty"`
	Observations  []Observation `yaml:"observations"`
	Tags          []string      `yaml:"tags,omitempty"`
	Timeout       time.Duration `yaml:"timeout,omitempty"`
	Retries       int           `yaml:"retries,omitempty"`
	RetryDelay    time.Duration `yaml:"retry_delay,omitempty"`
	RetryMaxDelay time.Duration `yaml:"retry_max_delay,omitempty"`
}

// LoopConfig represents the loop configuration in YAML.
type LoopConfig struct {
	Items string `yaml:"items"`        // Variable path, e.g., "{{ .vars.services }}"
	As    string `yaml:"as,omitempty"` // Optional custom variable name
}

// Observation represents an observation in YAML.
type Observation struct {
	Loop   *LoopConfig            `yaml:"loop,omitempty"`
	Plugin string                 `yaml:"plugin"`
	Config map[string]interface{} `yaml:"config,omitempty"`
	Expect []string               `yaml:"expect,omitempty"`
}

// ToEntity converts the controls section to a domain entity.
func (c *Controls) ToEntity() entities.ControlsSection {
	section := entities.ControlsSection{
		Items: make([]entities.Control, len(c.Items)),
	}

	if c.Defaults != nil {
		defaults := c.Defaults.ToEntity()
		section.Defaults = &defaults
	}

	for i, item := range c.Items {
		section.Items[i] = item.ToEntity()
	}

	return section
}

// ToEntity converts the defaults to a domain entity.
func (d *Defaults) ToEntity() entities.ControlDefaults {
	return entities.ControlDefaults{
		Severity:      d.Severity,
		Owner:         d.Owner,
		RetryBackoff:  entities.BackoffType(d.RetryBackoff),
		Tags:          d.Tags,
		Timeout:       d.Timeout,
		Retries:       d.Retries,
		RetryDelay:    d.RetryDelay,
		RetryMaxDelay: d.RetryMaxDelay,
	}
}

// ToEntity converts the control to a domain entity.
func (c *Control) ToEntity() entities.Control {
	ctrl := entities.Control{
		ID:                     c.ID,
		Name:                   c.Name,
		Description:            c.Description,
		Severity:               c.Severity,
		Owner:                  c.Owner,
		RetryBackoff:           entities.BackoffType(c.RetryBackoff),
		DependsOn:              c.DependsOn,
		Tags:                   c.Tags,
		Timeout:                c.Timeout,
		Retries:                c.Retries,
		RetryDelay:             c.RetryDelay,
		RetryMaxDelay:          c.RetryMaxDelay,
		ObservationDefinitions: make([]entities.ObservationDefinition, len(c.Observations)),
	}

	for i, obs := range c.Observations {
		ctrl.ObservationDefinitions[i] = obs.ToEntity()
	}

	return ctrl
}

// ToEntity converts the observation to a domain entity.
func (o *Observation) ToEntity() entities.ObservationDefinition {
	def := entities.ObservationDefinition{
		Plugin: o.Plugin,
		Config: o.Config,
		Expect: o.Expect,
	}
	if o.Loop != nil {
		def.Loop = &entities.LoopConfig{
			Items: o.Loop.Items,
			As:    o.Loop.As,
		}
	}
	return def
}
