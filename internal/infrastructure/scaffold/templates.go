// Package scaffold provides profile scaffolding functionality for the init command.
// It generates starter profiles and system configurations based on user-selected plugins.
package scaffold

// PluginExample represents a template for a single plugin's example control.
// Each plugin type has one example demonstrating its basic usage.
type PluginExample struct {
	// PluginName is the plugin identifier (file, http, etc.)
	PluginName string

	// ControlID is the unique identifier for the example control
	ControlID string

	// ControlName is the human-readable name for the control
	ControlName string

	// Description explains what the example demonstrates
	Description string

	// ConfigYAML is the plugin-specific configuration as a YAML fragment
	// This is embedded directly in the generated profile
	ConfigYAML string

	// ExpectExpressions are the example expect expressions
	ExpectExpressions []string

	// Capabilities lists the required capability grants for this example
	Capabilities []CapabilityGrant
}

// CapabilityGrant represents a single capability grant for config generation.
type CapabilityGrant struct {
	Kind    string // fs, network, exec, env
	Pattern string // e.g., "read:/etc/hostname", "outbound:80,443"
}

// PluginInfo provides metadata about available plugins.
type PluginInfo struct {
	Name        string // Plugin identifier
	Description string // User-facing description for selection UI
}

// AvailablePlugins returns the list of plugins that can be selected during init.
var AvailablePlugins = []PluginInfo{
	{Name: "file", Description: "Validate file existence, permissions, and content"},
	{Name: "http", Description: "Check HTTP endpoints and responses"},
	{Name: "dns", Description: "Verify DNS records and resolution"},
	{Name: "tcp", Description: "Test TCP port connectivity"},
	{Name: "command", Description: "Execute shell commands and validate output"},
	{Name: "smtp", Description: "Validate SMTP server configuration"},
}

// ValidPluginNames returns a set of valid plugin names for validation.
func ValidPluginNames() map[string]bool {
	names := make(map[string]bool, len(AvailablePlugins))
	for _, p := range AvailablePlugins {
		names[p.Name] = true
	}
	return names
}
