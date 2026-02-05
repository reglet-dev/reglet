// Package hostfuncs provides host functions for WASM plugins
package hostfuncs

import (
	"context"

	"github.com/reglet-dev/reglet-sdk/domain/entities"
	"github.com/reglet-dev/reglet-sdk/hostfuncs"
)

// CapabilityChecker is an alias to the SDK's CapabilityChecker.
// This allows Reglet to use the SDK's implementation while maintaining
// backward compatibility with existing code.
type CapabilityChecker = hostfuncs.CapabilityChecker

// NewCapabilityChecker creates a new capability checker using the SDK implementation.
func NewCapabilityChecker(caps map[string]*entities.GrantSet) *CapabilityChecker {
	return hostfuncs.NewCapabilityChecker(caps)
}

// Context helpers - delegate to SDK implementations

// WithPluginName adds the plugin name to the context
func WithPluginName(ctx context.Context, name string) context.Context {
	return hostfuncs.WithCapabilityPluginName(ctx, name)
}

// PluginNameFromContext retrieves the plugin name from the context
func PluginNameFromContext(ctx context.Context) (string, bool) {
	return hostfuncs.CapabilityPluginNameFromContext(ctx)
}
