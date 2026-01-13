// Package entities contains domain entities for the Reglet domain model.
package entities

// ProfileReader provides read-only access to profile data.
// This interface enforces immutability and prevents accidental mutations.
//
// Both raw Profile and ValidatedProfile implement this interface,
// allowing consumers to work with either type through the same contract.
type ProfileReader interface {
	// Metadata access
	GetMetadata() ProfileMetadata
	GetPlugins() []string
	GetVars() map[string]interface{}

	// Access to controls
	GetControls() ControlSet
}
