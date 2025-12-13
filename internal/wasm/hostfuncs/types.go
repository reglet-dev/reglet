// Package hostfuncs provides host functions for WASM plugins
package hostfuncs

import (
	"context"
	"fmt"

	"github.com/whiskeyjimbo/reglet/internal/domain/capabilities"
)

// CapabilityChecker checks if operations are allowed based on granted capabilities
type CapabilityChecker struct {
	policy              *capabilities.Policy
	grantedCapabilities map[string][]capabilities.Capability
}

// NewCapabilityChecker creates a new capability checker with the given capabilities
func NewCapabilityChecker(caps map[string][]capabilities.Capability) *CapabilityChecker {
	return &CapabilityChecker{
		policy:              capabilities.NewPolicy(),
		grantedCapabilities: caps,
	}
}

// Check verifies if a requested capability is granted for a specific plugin.
func (c *CapabilityChecker) Check(pluginName, kind, pattern string) error {
	requested := capabilities.Capability{Kind: kind, Pattern: pattern}
	pluginGrants, ok := c.grantedCapabilities[pluginName]
	if !ok {
		return fmt.Errorf("no capabilities granted to plugin %s", pluginName)
	}

	if c.policy.IsGranted(requested, pluginGrants) {
		return nil
	}

	return fmt.Errorf("capability denied: %s:%s", kind, pattern)
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