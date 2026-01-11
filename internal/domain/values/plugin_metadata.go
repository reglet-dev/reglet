package values

// PluginMetadata contains descriptive information about a plugin.
type PluginMetadata struct {
	name         string
	version      string
	description  string
	capabilities []string
}

// NewPluginMetadata creates plugin metadata.
func NewPluginMetadata(name, version, description string, capabilities []string) PluginMetadata {
	return PluginMetadata{
		name:         name,
		version:      version,
		description:  description,
		capabilities: capabilities,
	}
}

// Name returns the plugin name.
func (m PluginMetadata) Name() string {
	return m.name
}

// Version returns the semantic version.
func (m PluginMetadata) Version() string {
	return m.version
}

// Description returns human-readable description.
func (m PluginMetadata) Description() string {
	return m.description
}

// Capabilities returns required capabilities.
func (m PluginMetadata) Capabilities() []string {
	return m.capabilities
}
