// Package hostfuncs provides host functions for WASM plugins
package hostfuncs

import "context"

// Capability represents a permission requirement
// Defined here to avoid import cycle with internal/wasm
type Capability struct {
	Kind    string // fs, network, env, exec
	Pattern string // e.g., "/etc/**", "80,443", "AWS_*"
}

// CapabilityChecker checks if operations are allowed based on granted capabilities
type CapabilityChecker struct {
	// Map of plugin name to granted capabilities
	grantedCapabilities map[string][]Capability
}

// NewCapabilityChecker creates a new capability checker with the given capabilities
func NewCapabilityChecker(caps map[string][]Capability) *CapabilityChecker {
	return &CapabilityChecker{
		grantedCapabilities: caps,
	}
}

type contextKey struct {
	name string
}

var pluginNameKey = &contextKey{name: "plugin_name"}

// WithPluginName adds the plugin name to the context
func WithPluginName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, pluginNameKey, name)
}

// PluginNameFromContext retrieves the plugin name from the context
func PluginNameFromContext(ctx context.Context) (string, bool) {
	name, ok := ctx.Value(pluginNameKey).(string)
	return name, ok
}
