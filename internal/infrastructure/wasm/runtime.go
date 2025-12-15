package wasm

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/whiskeyjimbo/reglet/internal/domain/capabilities"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/build"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/wasm/hostfuncs"
)

// globalCache speeds up compilation across runtimes.
var globalCache = wazero.NewCompilationCache()

// Runtime manages WASM execution.
type Runtime struct {
	runtime wazero.Runtime
	plugins map[string]*Plugin // Loaded plugins by name
	version build.Info
}

// NewRuntime creates a runtime with no capabilities.
func NewRuntime(ctx context.Context, version build.Info) (*Runtime, error) {
	return NewRuntimeWithCapabilities(ctx, version, nil)
}

// NewRuntimeWithCapabilities initializes runtime with permissions.
func NewRuntimeWithCapabilities(ctx context.Context, version build.Info, caps map[string][]capabilities.Capability) (*Runtime, error) {
	// Create pure Go WASM runtime with compilation cache.
	config := wazero.NewRuntimeConfig().WithCompilationCache(globalCache)
	r := wazero.NewRuntimeWithConfig(ctx, config)

	// Instantiate WASI for system calls (clock, random, etc.).
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
		_ = r.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate WASI: %w", err)
	}

	// Register host functions with capability enforcement.
	if err := hostfuncs.RegisterHostFunctions(ctx, r, version, caps); err != nil {
		_ = r.Close(ctx)
		return nil, fmt.Errorf("failed to register host functions: %w", err)
	}

	return &Runtime{
		runtime: r,
		plugins: make(map[string]*Plugin),
		version: version,
	}, nil
}

// LoadPlugin compiles and caches a plugin.
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
