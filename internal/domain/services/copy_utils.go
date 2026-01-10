// Package services contains domain services for the Reglet domain model.
// These are stateless services that encapsulate business logic.
package services

import (
	"github.com/reglet-dev/reglet/internal/domain/entities"
)

// ===== DEEP COPY UTILITIES =====
//
// These functions provide deep copying of profile-related structures.
// They ensure immutability by creating independent copies that don't share references.
// Used by ProfileCompiler and ProfileMerger.

// DeepCopyProfile creates a complete deep copy of a profile.
// This ensures the original profile remains unchanged during compilation or merging.
func DeepCopyProfile(original *entities.Profile) *entities.Profile {
	if original == nil {
		return nil
	}

	return &entities.Profile{
		Metadata: original.Metadata, // ProfileMetadata is a value type (copied automatically)
		Plugins:  CopyStringSlice(original.Plugins),
		Vars:     CopyVars(original.Vars),
		Controls: entities.ControlsSection{
			Defaults: CopyDefaults(original.Controls.Defaults),
			Items:    CopyControls(original.Controls.Items),
		},
		Extends: CopyStringSlice(original.Extends),
	}
}

// CopyStringSlice creates a deep copy of a string slice.
func CopyStringSlice(src []string) []string {
	if src == nil {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

// CopyVars creates a shallow copy of a vars map.
// Note: Values are interface{} and cannot be deep copied generically.
// For most use cases (strings, numbers, bools), this is sufficient.
func CopyVars(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v // Shallow copy of values (interface{} can't be deep copied generically)
	}
	return dst
}

// CopyDefaults creates a deep copy of control defaults.
func CopyDefaults(src *entities.ControlDefaults) *entities.ControlDefaults {
	if src == nil {
		return nil
	}
	return &entities.ControlDefaults{
		Severity: src.Severity,
		Owner:    src.Owner,
		Tags:     CopyStringSlice(src.Tags),
		Timeout:  src.Timeout,
	}
}

// CopyControls creates a deep copy of a controls slice.
func CopyControls(src []entities.Control) []entities.Control {
	if src == nil {
		return nil
	}
	dst := make([]entities.Control, len(src))
	for i, ctrl := range src {
		dst[i] = entities.Control{
			ID:                     ctrl.ID,
			Name:                   ctrl.Name,
			Description:            ctrl.Description,
			Severity:               ctrl.Severity,
			Owner:                  ctrl.Owner,
			Tags:                   CopyStringSlice(ctrl.Tags),
			DependsOn:              CopyStringSlice(ctrl.DependsOn),
			Timeout:                ctrl.Timeout,
			ObservationDefinitions: CopyObservations(ctrl.ObservationDefinitions),
		}
	}
	return dst
}

// CopyObservations creates a deep copy of observation definitions.
func CopyObservations(src []entities.ObservationDefinition) []entities.ObservationDefinition {
	if src == nil {
		return nil
	}
	dst := make([]entities.ObservationDefinition, len(src))
	for i, obs := range src {
		dst[i] = entities.ObservationDefinition{
			Plugin: obs.Plugin,
			Config: CopyConfig(obs.Config),
			Expect: CopyStringSlice(obs.Expect),
		}
	}
	return dst
}

// CopyConfig creates a shallow copy of a config map.
// Note: Values are interface{} and cannot be deep copied generically.
func CopyConfig(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v // Shallow copy (can't deep copy interface{} generically)
	}
	return dst
}
