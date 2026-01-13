// Package entities contains domain entities for the Reglet domain model.
// These are pure domain types with NO infrastructure dependencies.
package entities

import (
	"fmt"
)

// Profile represents the Reglet profile configuration.
// It serves as the aggregate root for the configuration context, defining the
// validation configuration and ruleset.
//
// Invariants enforced:
// - Unique control IDs
// - All dependencies must exist
// - Name and version are mandatory
// - At least one observation per control
type Profile struct {
	Metadata ProfileMetadata
	Plugins  []string
	Vars     map[string]interface{}
	Controls ControlsSection

	// Extends specifies parent profiles to inherit from.
	// Multiple parents are merged left-to-right before applying current profile.
	// This field is NOT propagated after merge resolution.
	Extends []string
}

// ProfileMetadata contains descriptive information about the profile.
type ProfileMetadata struct {
	Name        string
	Version     string
	Description string
}

// ControlsSection groups validation controls and their default settings.
type ControlsSection struct {
	Defaults *ControlDefaults
	Items    ControlSet
}

// ===== PROFILE AGGREGATE ROOT METHODS =====

// GetMetadata returns the profile metadata.
func (p *Profile) GetMetadata() ProfileMetadata {
	return p.Metadata
}

// GetPlugins returns the list of plugins required by this profile.
func (p *Profile) GetPlugins() []string {
	return p.Plugins
}

// GetVars returns the profile variables.
func (p *Profile) GetVars() map[string]interface{} {
	return p.Vars
}

// GetAllControls returns all controls in the profile.
func (p *Profile) GetAllControls() []Control {
	return p.Controls.Items
}

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

	return p.Controls.Items.Validate()
}

// AddControl safely adds a new control to the profile.
// It returns an error if the control is invalid or already exists.
func (p *Profile) AddControl(ctrl Control) error {
	newSet, err := p.Controls.Items.Add(ctrl)
	if err != nil {
		return err
	}
	p.Controls.Items = newSet
	return nil
}

// GetControls returns the set of controls in the profile.
func (p *Profile) GetControls() ControlSet {
	return p.Controls.Items
}
