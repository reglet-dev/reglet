package config

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
)

// Control ID must be alphanumeric with dashes and underscores
var controlIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// PluginSchemaProvider is an interface for loading plugins and retrieving their schemas.
// This allows validation code to be decoupled from the WASM runtime implementation.
type PluginSchemaProvider interface {
	// GetPluginSchema returns the JSON Schema for a plugin's configuration.
	// Returns nil if the plugin is not found.
	GetPluginSchema(ctx context.Context, pluginName string) ([]byte, error)
}

// Validate performs comprehensive validation of a profile.
// Returns an error describing all validation failures found.
// This performs structural validation only - no schema validation.
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

// ValidateWithSchemas performs comprehensive validation including plugin config schema validation.
// This requires a PluginSchemaProvider to fetch plugin schemas during validation.
// Use this for pre-flight validation before execution.
func ValidateWithSchemas(ctx context.Context, profile *Profile, provider PluginSchemaProvider) error {
	// First run basic structural validation
	if err := Validate(profile); err != nil {
		return err
	}

	// Then validate each observation's config against its plugin's schema
	var errors []string
	for _, ctrl := range profile.Controls.Items {
		for j, obs := range ctrl.Observations {
			if err := validateObservationSchema(ctx, obs, provider); err != nil {
				errors = append(errors, fmt.Sprintf("control %s, observation %d (%s): %s", ctrl.ID, j, obs.Plugin, err.Error()))
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("schema validation failed:\n  - %s", strings.Join(errors, "\n  - "))
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

// validateObservationSchema validates an observation's config against its plugin's JSON Schema.
func validateObservationSchema(ctx context.Context, obs Observation, provider PluginSchemaProvider) error {
	// Get the plugin's schema
	schemaBytes, err := provider.GetPluginSchema(ctx, obs.Plugin)
	if err != nil {
		return fmt.Errorf("failed to get schema for plugin %s: %w", obs.Plugin, err)
	}

	if len(schemaBytes) == 0 {
		// No schema available - skip validation
		// This can happen if the plugin doesn't provide a schema
		return nil
	}

	// Compile the schema
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft2020

	// Add the schema document from bytes
	if err := compiler.AddResource("schema.json", bytes.NewReader(schemaBytes)); err != nil {
		return fmt.Errorf("failed to add schema resource for plugin %s: %w", obs.Plugin, err)
	}

	schema, err := compiler.Compile("schema.json")
	if err != nil {
		return fmt.Errorf("failed to compile schema for plugin %s: %w", obs.Plugin, err)
	}

	// Validate the observation config against the schema
	if err := schema.Validate(obs.Config); err != nil {
		// Format validation errors nicely
		if validationErr, ok := err.(*jsonschema.ValidationError); ok {
			return formatSchemaValidationError(validationErr)
		}
		return fmt.Errorf("config validation failed: %w", err)
	}

	return nil
}

// formatSchemaValidationError formats a JSON Schema validation error into a readable message.
func formatSchemaValidationError(err *jsonschema.ValidationError) error {
	var messages []string

	// Collect all validation errors
	var collectErrors func(*jsonschema.ValidationError)
	collectErrors = func(e *jsonschema.ValidationError) {
		// If this error has a message, add it
		if e.Message != "" {
			location := e.InstanceLocation
			if location == "" {
				location = "(root)"
			}
			messages = append(messages, fmt.Sprintf("%s: %s", location, e.Message))
		}

		// Recurse into causes
		for _, cause := range e.Causes {
			collectErrors(cause)
		}
	}

	collectErrors(err)

	if len(messages) == 0 {
		return fmt.Errorf("validation failed")
	}

	return fmt.Errorf("config validation failed:\n    - %s", strings.Join(messages, "\n    - "))
}

// isValidVersion checks if a version string is valid semver format.
// This is a simple check for Phase 1b - just verify X.Y.Z format.
func isValidVersion(version string) bool {
	// Simple regex for basic semver: X.Y.Z where X, Y, Z are numbers
	versionPattern := regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	return versionPattern.MatchString(version)
}
