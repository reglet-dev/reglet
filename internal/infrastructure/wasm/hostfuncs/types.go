// Package hostfuncs provides host functions for WASM plugins
package hostfuncs

import (
	"context"

	"github.com/reglet-dev/reglet-abi/hostfunc"
	hostlib "github.com/reglet-dev/reglet-hostlib"
)

// CapabilityChecker is an alias to the SDK's CapabilityChecker.
// This allows Reglet to use the SDK's implementation while maintaining
// backward compatibility with existing code.
type CapabilityChecker = hostlib.CapabilityChecker

// NewCapabilityChecker creates a new capability checker using the SDK implementation.
func NewCapabilityChecker(caps map[string]*hostfunc.GrantSet) *CapabilityChecker {
	return hostlib.NewCapabilityChecker(caps)
}

// Context helpers - delegate to SDK implementations

// WithPluginName adds the plugin name to the context
func WithPluginName(ctx context.Context, name string) context.Context {
	return hostlib.WithCapabilityPluginName(ctx, name)
}

// PluginNameFromContext retrieves the plugin name from the context
func PluginNameFromContext(ctx context.Context) (string, bool) {
	return hostlib.CapabilityPluginNameFromContext(ctx)
}
