package ports

import (
	"context"

	abi "github.com/reglet-dev/reglet-abi"
	"github.com/reglet-dev/reglet/internal/domain/capability"
)

// Plugin represents a loaded WASM plugin that can be inspected and executed.
// This interface abstracts the concrete wasm.Plugin implementation.
type Plugin interface {
	// Manifest returns plugin metadata including declared capabilities.
	Manifest(ctx context.Context) (*abi.Manifest, error)

	// RequiredCapabilities returns the capabilities declared in the plugin manifest
	// converted to internal domain types.
	RequiredCapabilities(ctx context.Context) (capability.GrantSet, error)
}

// PluginRuntime abstracts the WASM runtime for plugin loading and management.
// This interface allows the application layer to work with plugins without
// depending on concrete infrastructure types like wasm.Runtime.
type PluginRuntime interface {
	// LoadPlugin compiles and caches a plugin from WASM bytes.
	LoadPlugin(ctx context.Context, name string, wasmBytes []byte) (Plugin, error)

	// Close releases runtime resources.
	Close(ctx context.Context) error
}

// RuntimeOption configures runtime creation.
// This type alias allows the application layer to pass options without importing infrastructure.
type RuntimeOption any

// PluginRuntimeFactory creates runtime instances.
// This allows the application layer to create runtimes without importing infrastructure.
type PluginRuntimeFactory interface {
	// NewRuntime creates a new plugin runtime with optional configuration.
	// Accepts functional options for capabilities, redaction, memory limits, and caching.
	NewRuntime(ctx context.Context, opts ...RuntimeOption) (PluginRuntime, error)
}
