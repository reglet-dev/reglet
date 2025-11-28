package wasm

import (
	"context"
	"fmt"

	"github.com/jrose/reglet/internal/wasm/hostfuncs"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

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
func NewRuntimeWithCapabilities(ctx context.Context, caps []hostfuncs.Capability) (*Runtime, error) {
	// Create wazero runtime with default configuration
	// This is a pure Go WASM runtime - no CGO required
	r := wazero.NewRuntime(ctx)

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

// Close closes the runtime and cleans up resources
func (r *Runtime) Close(ctx context.Context) error {
	return r.runtime.Close(ctx)
}
