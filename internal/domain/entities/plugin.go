package entities

import (
	"github.com/reglet-dev/reglet/internal/domain/values"
)

// Plugin is the aggregate root for the Plugin Management bounded context.
// Represents a WASM plugin with verified integrity and metadata.
type Plugin struct {
	reference values.PluginReference
	digest    values.Digest
	metadata  values.PluginMetadata
}

// NewPlugin creates a new plugin entity.
func NewPlugin(
	ref values.PluginReference,
	digest values.Digest,
	metadata values.PluginMetadata,
) *Plugin {
	return &Plugin{
		reference: ref,
		digest:    digest,
		metadata:  metadata,
	}
}

// Reference returns the plugin's unique identifier.
func (p *Plugin) Reference() values.PluginReference {
	return p.reference
}

// Digest returns the plugin's content hash.
func (p *Plugin) Digest() values.Digest {
	return p.digest
}

// Metadata returns the plugin's descriptive information.
func (p *Plugin) Metadata() values.PluginMetadata {
	return p.metadata
}

// VerifyIntegrity checks if the plugin's digest matches expected value.
func (p *Plugin) VerifyIntegrity(expected values.Digest) error {
	if !p.digest.Equals(expected) {
		return &IntegrityError{
			Expected: expected,
			Actual:   p.digest,
		}
	}
	return nil
}
