package wasm

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/whiskeyjimbo/reglet/internal/wasm/hostfuncs"
)

// globalCache is a shared compilation cache for wazero runtimes.
// This significantly speeds up compilation when creating multiple runtimes
// (e.g. during testing or parallel execution).
var globalCache = wazero.NewCompilationCache()

// Runtime manages the WASM runtime environment
type Runtime struct {
	runtime wazero.Runtime
	plugins map[string]*Plugin // Loaded plugins by name
}

// NewRuntime creates a new WASM runtime with no granted capabilities
// For production use, use NewRuntimeWithCapabilities instead
func NewRuntime(ctx context.Context) (*Runtime, error) {
	return NewRuntimeWithCapabilities(ctx, nil)
}

// NewRuntimeWithCapabilities creates a new WASM runtime with specific capabilities
func NewRuntimeWithCapabilities(ctx context.Context, caps map[string][]hostfuncs.Capability) (*Runtime, error) {
	// Create wazero runtime with compilation cache
	// This is a pure Go WASM runtime - no CGO required
	config := wazero.NewRuntimeConfig().WithCompilationCache(globalCache)
	r := wazero.NewRuntimeWithConfig(ctx, config)

	// Instantiate WASI to support standard system calls
	// Plugins may need basic WASI functions (clock, random, etc.)
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
		_ = r.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate WASI: %w", err)
	}

	// Register custom host functions with capability enforcement
	// Capabilities define what operations plugins are allowed to perform
	if err := hostfuncs.RegisterHostFunctions(ctx, r, caps); err != nil {
		_ = r.Close(ctx)
		return nil, fmt.Errorf("failed to register host functions: %w", err)
	}

	return &Runtime{
		runtime: r,
		plugins: make(map[string]*Plugin),
	}, nil
}

// LoadPlugin loads a WASM plugin from bytes
func (r *Runtime) LoadPlugin(ctx context.Context, name string, wasmBytes []byte) (*Plugin, error) {
	// Check if plugin is already loaded
	if p, ok := r.plugins[name]; ok {
		return p, nil
	}

	// Compile the WASM module
	compiledModule, err := r.runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to compile plugin %s: %w", name, err)
	}

	// Create plugin wrapper
	plugin := &Plugin{
		name:    name,
		module:  compiledModule,
		runtime: r.runtime,
	}

	// Cache the plugin
	r.plugins[name] = plugin

	return plugin, nil
}

// GetPlugin retrieves a loaded plugin by name
func (r *Runtime) GetPlugin(name string) (*Plugin, bool) {
	p, ok := r.plugins[name]
	return p, ok
}

// GetPluginSchema implements config.PluginSchemaProvider.
// It loads the plugin (if not already loaded) and retrieves its JSON Schema.
func (r *Runtime) GetPluginSchema(ctx context.Context, pluginName string) ([]byte, error) {
	// Check if plugin is already loaded
	plugin, ok := r.plugins[pluginName]
	if !ok {
		// Plugin not loaded - need to load it first
		// This requires finding the plugin WASM file
		// For now, return an error indicating the plugin needs to be loaded
		return nil, fmt.Errorf("plugin %s not loaded; call LoadPlugin first", pluginName)
	}

	// Get the schema from the plugin
	schema, err := plugin.Schema(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema for plugin %s: %w", pluginName, err)
	}

	if schema == nil || len(schema.RawSchema) == 0 {
		// Plugin doesn't provide a schema
		return nil, nil
	}

	return schema.RawSchema, nil
}

// Close closes the runtime and cleans up resources
func (r *Runtime) Close(ctx context.Context) error {
	return r.runtime.Close(ctx)
}
