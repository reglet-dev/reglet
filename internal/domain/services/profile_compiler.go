package services

import (
	"fmt"

	"github.com/reglet-dev/reglet/internal/domain/entities"
)

// ProfileCompiler transforms raw profiles into validated, immutable profiles.
// This is a domain service that encapsulates the compilation process.
//
// Compilation steps:
// 1. Deep copy the raw profile (prevent mutation)
// 2. Apply default values to controls
// 3. Validate invariants
// 4. Return immutable ValidatedProfile
type ProfileCompiler struct{}

// NewProfileCompiler creates a new profile compiler service.
func NewProfileCompiler() *ProfileCompiler {
	return &ProfileCompiler{}
}

// Compile transforms a raw profile into a validated, immutable profile.
// The input profile is NOT modified (immutability guarantee).
//
// Returns an error if the profile fails validation.
func (c *ProfileCompiler) Compile(raw *entities.Profile) (*entities.ValidatedProfile, error) {
	if raw == nil {
		return nil, fmt.Errorf("cannot compile nil profile")
	}

	// Step 1: Deep copy to prevent mutation of the original
	compiled := DeepCopyProfile(raw)

	// Step 2: Apply defaults (business rule)
	c.applyDefaults(compiled)

	// Step 3: Validate invariants
	if err := compiled.Validate(); err != nil {
		return nil, fmt.Errorf("profile validation failed: %w", err)
	}

	// Step 4: Create immutable ValidatedProfile
	return entities.NewValidatedProfile(compiled), nil
}

// applyDefaults propagates default values to all controls.
// This is the non-mutating version of Profile.ApplyDefaults().
func (c *ProfileCompiler) applyDefaults(profile *entities.Profile) {
	if profile.Controls.Defaults == nil {
		return
	}

	defaults := profile.Controls.Defaults

	for i := range profile.Controls.Items {
		ctrl := &profile.Controls.Items[i]

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
