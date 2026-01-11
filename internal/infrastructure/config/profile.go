package config

import (
	"github.com/reglet-dev/reglet/internal/domain/entities"
)

// Profile represents the YAML structure of a profile.
type Profile struct {
	Metadata Metadata               `yaml:"profile"`
	Plugins  []string               `yaml:"plugins,omitempty"`
	Vars     map[string]interface{} `yaml:"vars,omitempty"`
	Controls Controls               `yaml:"controls"`
	Extends  []string               `yaml:"extends,omitempty"`
}

// Metadata represents the metadata section in YAML.
type Metadata struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Description string `yaml:"description,omitempty"`
}

// ToEntity converts the config representation to a domain Profile entity.
func (p *Profile) ToEntity() entities.Profile {
	profile := entities.Profile{
		Metadata: p.Metadata.ToEntity(),
		Plugins:  p.Plugins,
		Vars:     p.Vars,
		Controls: p.Controls.ToEntity(),
		Extends:  p.Extends,
	}
	return profile
}

// ToEntity converts the metadata to a domain entity.
func (m *Metadata) ToEntity() entities.ProfileMetadata {
	return entities.ProfileMetadata{
		Name:        m.Name,
		Version:     m.Version,
		Description: m.Description,
	}
}
