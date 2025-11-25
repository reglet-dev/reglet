package config

import (
	"fmt"
	"regexp"
	"strings"
)

// Control ID must be alphanumeric with dashes and underscores
var controlIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Validate performs comprehensive validation of a profile.
// Returns an error describing all validation failures found.
func Validate(profile *Profile) error {
	var errors []string

	// Validate metadata
	if err := validateMetadata(profile.Metadata); err != nil {
		errors = append(errors, err.Error())
	}

	// Validate controls
	if err := validateControls(profile.Controls); err != nil {
		errors = append(errors, err.Error())
	}

	if len(errors) > 0 {
		return fmt.Errorf("profile validation failed:\n  - %s", strings.Join(errors, "\n  - "))
	}

	return nil
}

// validateMetadata validates profile metadata fields.
func validateMetadata(meta ProfileMetadata) error {
	var errors []string

	if meta.Name == "" {
		errors = append(errors, "profile name is required")
	}

	if meta.Version == "" {
		errors = append(errors, "profile version is required")
	}

	// Basic semver validation (simple check for Phase 1b)
	if meta.Version != "" && !isValidVersion(meta.Version) {
		errors = append(errors, fmt.Sprintf("profile version %q is not valid (expected format: X.Y.Z)", meta.Version))
	}

	if len(errors) > 0 {
		return fmt.Errorf("profile metadata: %s", strings.Join(errors, "; "))
	}

	return nil
}

// validateControls validates the controls section.
func validateControls(controls ControlsSection) error {
	if len(controls.Items) == 0 {
		return fmt.Errorf("at least one control is required")
	}

	// Track control IDs to detect duplicates
	controlIDs := make(map[string]bool)

	var errors []string

	for i, ctrl := range controls.Items {
		if err := validateControl(ctrl); err != nil {
			errors = append(errors, fmt.Sprintf("control %d: %s", i, err.Error()))
		}

		// Check for duplicate control IDs
		if controlIDs[ctrl.ID] {
			errors = append(errors, fmt.Sprintf("duplicate control ID: %s", ctrl.ID))
		}
		controlIDs[ctrl.ID] = true
	}

	if len(errors) > 0 {
		return fmt.Errorf("controls validation:\n    - %s", strings.Join(errors, "\n    - "))
	}

	return nil
}

// validateControl validates a single control.
func validateControl(ctrl Control) error {
	var errors []string

	// ID is required and must be valid format
	if ctrl.ID == "" {
		errors = append(errors, "control ID is required")
	} else if !controlIDPattern.MatchString(ctrl.ID) {
		errors = append(errors, fmt.Sprintf("control ID %q is invalid (must be alphanumeric with dashes/underscores)", ctrl.ID))
	}

	// Name is required
	if ctrl.Name == "" {
		errors = append(errors, "control name is required")
	}

	// At least one observation is required
	if len(ctrl.Observations) == 0 {
		errors = append(errors, "at least one observation is required")
	}

	// Validate each observation
	for j, obs := range ctrl.Observations {
		if err := validateObservation(obs); err != nil {
			errors = append(errors, fmt.Sprintf("observation %d: %s", j, err.Error()))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("%s", strings.Join(errors, "; "))
	}

	return nil
}

// validateObservation validates a single observation.
func validateObservation(obs Observation) error {
	var errors []string

	// Plugin is required
	if obs.Plugin == "" {
		errors = append(errors, "plugin name is required")
	}

	// Config is required (even if empty map)
	if obs.Config == nil {
		errors = append(errors, "config is required")
	}

	if len(errors) > 0 {
		return fmt.Errorf("%s", strings.Join(errors, "; "))
	}

	return nil
}

// isValidVersion checks if a version string is valid semver format.
// This is a simple check for Phase 1b - just verify X.Y.Z format.
func isValidVersion(version string) bool {
	// Simple regex for basic semver: X.Y.Z where X, Y, Z are numbers
	versionPattern := regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	return versionPattern.MatchString(version)
}
