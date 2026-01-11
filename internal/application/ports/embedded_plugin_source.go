package ports

// EmbeddedPluginSource provides access to built-in WASM plugins.
type EmbeddedPluginSource interface {
	// Get returns the file path to an embedded plugin.
	// Returns empty string if not found.
	Get(name string) string

	// List returns names of all embedded plugins.
	List() []string
}
