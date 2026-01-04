// Package entities contains domain entities for the Reglet domain model.
package entities

// ValidatedProfile represents a fully compiled and validated profile.
// This is an immutable value object created by the ProfileCompiler.
//
// It embeds the raw Profile and adds compiled/enriched state:
// - Defaults have been applied to all controls
// - All validations have passed
// - Dependency graph has been verified (no cycles)
type ValidatedProfile struct {
	*Profile // Embedded raw profile (provides ProfileReader interface)

	// Compiled state - computed at compilation time, immutable afterward
	// Future: could add execution order, resolved variables, etc.
}

// NewValidatedProfile creates a new ValidatedProfile from a raw profile.
// This is an internal constructor - use ProfileCompiler.Compile() instead.
func NewValidatedProfile(profile *Profile) *ValidatedProfile {
	return &ValidatedProfile{
		Profile: profile,
	}
}

// IsValidated always returns true for ValidatedProfile.
// This is a marker method to distinguish from raw Profile at runtime if needed.
func (v *ValidatedProfile) IsValidated() bool {
	return true
}

// Note: ValidatedProfile inherits all ProfileReader methods from embedded *Profile
// it automatically implements the ProfileReader interface.
