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
	BuildPluginRegistry() (*PluginRegistry, error)
	GetVars() map[string]interface{}

	// Control queries
	GetControl(id string) *Control
	HasControl(id string) bool
	ControlCount() int
	GetAllControls() []Control

	// Filtering
	SelectControlsByTags(tags []string) []Control
	SelectControlsBySeverity(severities []string) []Control
	ExcludeControlsByID(excludeIDs []string) []Control

	// Validation - control dependency cycle detection (NOT profile inheritance cycles)
	CheckForControlDependencyCycles() error
}
