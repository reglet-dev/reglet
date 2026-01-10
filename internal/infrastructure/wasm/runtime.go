package wasm

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/reglet-dev/reglet/internal/domain/capabilities"
	"github.com/reglet-dev/reglet/internal/infrastructure/build"
	"github.com/reglet-dev/reglet/internal/infrastructure/sensitivedata"
	"github.com/reglet-dev/reglet/internal/infrastructure/wasm/hostfuncs"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// globalCache speeds up compilation across runtimes within a single process.
//
// Cleanup considerations:
//   - CLI tools: No explicit cleanup needed - OS reclaims memory on exit.
//   - Servers/Workers: Call CloseGlobalCache() during graceful shutdown to
//     release resources before the process exits.
var globalCache = wazero.NewCompilationCache()

// CloseGlobalCache releases resources held by the global compilation cache.
// This is only needed for long-running processes (servers, workers) that require
// graceful shutdown. CLI tools can skip this - the OS handles cleanup on exit.
func CloseGlobalCache(ctx context.Context) error {
	return globalCache.Close(ctx)
}

// Runtime manages WASM execution.
type Runtime struct {
	runtime             wazero.Runtime
	plugins             map[string]*Plugin
	redactor            *sensitivedata.Redactor
	grantedCapabilities map[string][]capabilities.Capability
	version             build.Info
	frozenEnv           []string
	mu                  sync.RWMutex
}

// NewRuntime creates a runtime with no capabilities and no redaction.
func NewRuntime(ctx context.Context, version build.Info) (*Runtime, error) {
	return NewRuntimeWithCapabilities(ctx, version, nil, nil, 0)
}

// NewRuntimeWithCapabilities initializes runtime with permissions and optional output redaction.
func NewRuntimeWithCapabilities(
	ctx context.Context,
	version build.Info,
	caps map[string][]capabilities.Capability,
	redactor *sensitivedata.Redactor,
	memoryLimitMB int,
) (*Runtime, error) {
	// Determine memory limit
	// 0 = default (256MB)
	// -1 = unlimited
	// >0 = explicit limit in MB
	switch {
	case memoryLimitMB == 0:
		memoryLimitMB = 256 // Default: 256MB
		slog.Info("using default WASM memory limit", "mb", memoryLimitMB)
	case memoryLimitMB == -1:
		slog.Warn("WASM memory limit disabled (unlimited memory)")
		// Pass to wazero as is (unlimited)
	case memoryLimitMB > 0:
		if memoryLimitMB < 64 {
			slog.Warn("WASM memory limit very low, plugins may fail", "mb", memoryLimitMB)
		}
	default:
		return nil, fmt.Errorf("invalid WASM memory limit: %d (must be >= -1)", memoryLimitMB)
	}

	// Create pure Go WASM runtime with compilation cache.
	config := wazero.NewRuntimeConfig().WithCompilationCache(globalCache)

	// Apply memory limit if not unlimited
	if memoryLimitMB > 0 {
		// Convert MB to pages (1 page = 64KB)
		// 1 MB = 1024 KB = 16 * 64KB
		pages := uint32(memoryLimitMB * 16) //nolint:gosec // G115: memoryLimitMB is validated (max ~134M pages, well under uint32)
		config = config.WithMemoryLimitPages(pages)
	}

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
		runtime:             r,
		plugins:             make(map[string]*Plugin),
		version:             version,
		redactor:            redactor,
		grantedCapabilities: caps,
		frozenEnv:           os.Environ(), // Freeze environment at startup for security
	}, nil
}

// LoadPlugin compiles and caches a plugin.
func (r *Runtime) LoadPlugin(ctx context.Context, name string, wasmBytes []byte) (*Plugin, error) {
	// Fast path: Check if plugin is already loaded
	r.mu.RLock()
	if p, ok := r.plugins[name]; ok {
		r.mu.RUnlock()
		return p, nil
	}
	r.mu.RUnlock()

	// Slow path: Need to compile and load the plugin (write lock)
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check: Another goroutine may have loaded it while we waited for the lock
	if p, ok := r.plugins[name]; ok {
		return p, nil
	}

	// Compile the WASM module
	compiledModule, err := r.runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to compile plugin %s: %w", name, err)
	}

	// Create output writers with optional redaction
	var stdout, stderr io.Writer = os.Stderr, os.Stderr
	if r.redactor != nil {
		// Wrap os.Stderr with redaction to prevent secret leakage
		stdout = sensitivedata.NewWriter(os.Stderr, r.redactor)
		stderr = sensitivedata.NewWriter(os.Stderr, r.redactor)
	}

	// Create plugin wrapper
	plugin := &Plugin{
		name:         name,
		module:       compiledModule,
		runtime:      r.runtime,
		stdout:       stdout,
		stderr:       stderr,
		capabilities: r.grantedCapabilities[name], // Extract plugin-specific capabilities
		frozenEnv:    r.frozenEnv,                 // Pass frozen environment snapshot (prevents runtime env leakage)
	}

	// Cache the plugin
	r.plugins[name] = plugin

	return plugin, nil
}

// GetPlugin retrieves a loaded plugin by name.
func (r *Runtime) GetPlugin(name string) (*Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	return p, ok
}

// GetPluginSchema implements config.PluginSchemaProvider.
// It loads the plugin (if not already loaded) and retrieves its JSON Schema.
func (r *Runtime) GetPluginSchema(ctx context.Context, pluginName string) ([]byte, error) {
	// Check if plugin is already loaded
	r.mu.RLock()
	plugin, ok := r.plugins[pluginName]
	r.mu.RUnlock()

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
