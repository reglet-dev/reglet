package wasm

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet/internal/infrastructure/build"
	"github.com/reglet-dev/reglet/internal/infrastructure/sensitivedata"
	"github.com/reglet-dev/reglet/internal/infrastructure/wasm/hostfuncs"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// defaultGlobalCache is the shared compilation cache used when no custom cache is provided.
// This speeds up compilation across runtimes within a single process AND across runs.
//
// Uses disk-based caching (~/.cache/reglet/wasm) for persistence between test runs,
// significantly speeding up repeated test executions with the -race flag.
//
// Cleanup considerations:
//   - CLI tools: No explicit cleanup needed - OS reclaims memory on exit.
//   - Servers/Workers: Manage your own cache with WithCompilationCache() option.
//   - Tests: Benefits from disk cache across runs; use WithCompilationCache() for isolation.
var defaultGlobalCache = initGlobalCache()

func initGlobalCache() wazero.CompilationCache {
	// Try to use disk-based cache for persistence across test runs
	cacheDir := getCacheDir()
	if cacheDir != "" {
		cache, err := wazero.NewCompilationCacheWithDir(cacheDir)
		if err == nil {
			return cache
		}
		slog.Debug("failed to create disk compilation cache, using in-memory", "error", err)
	}
	return wazero.NewCompilationCache()
}

func getCacheDir() string {
	// Try XDG cache first, then fallback to home directory
	if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
		return filepath.Join(xdgCache, "reglet", "wasm")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".cache", "reglet", "wasm")
	}
	return ""
}

// Runtime manages WASM execution.
type Runtime struct {
	runtime             wazero.Runtime
	plugins             map[string]*Plugin
	redactor            *sensitivedata.Redactor
	grantedCapabilities map[string]*entities.GrantSet
	version             build.Info
	frozenEnv           []string
	mu                  sync.RWMutex
}

// RuntimeOption configures a Runtime.
type RuntimeOption func(*runtimeConfig)

// runtimeConfig holds configuration for runtime creation.
type runtimeConfig struct {
	cache         wazero.CompilationCache
	caps          map[string]*entities.GrantSet
	redactor      *sensitivedata.Redactor
	memoryLimitMB int
}

// WithCapabilities sets the granted capabilities using the SDK GrantSet format.
func WithCapabilities(caps map[string]*entities.GrantSet) RuntimeOption {
	return func(c *runtimeConfig) {
		c.caps = caps
	}
}

// WithRedactor enables secret redaction for plugin output.
func WithRedactor(redactor *sensitivedata.Redactor) RuntimeOption {
	return func(c *runtimeConfig) {
		c.redactor = redactor
	}
}

// WithMemoryLimit sets the WASM memory limit in MB.
// 0 = default (256MB), -1 = unlimited, >0 = explicit limit.
func WithMemoryLimit(mb int) RuntimeOption {
	return func(c *runtimeConfig) {
		c.memoryLimitMB = mb
	}
}

// WithCompilationCache provides a custom compilation cache for this runtime.
// This is useful for:
//   - Tests: Isolate cache between tests to prevent interference
//   - Servers: Multiple isolated runtime pools with separate caches
//   - Advanced use cases: Custom cache lifecycle management
//
// If not provided, uses the default shared cache for the process.
func WithCompilationCache(cache wazero.CompilationCache) RuntimeOption {
	return func(c *runtimeConfig) {
		c.cache = cache
	}
}

// NewRuntime creates a runtime with optional configuration.
// By default, creates a runtime with no capabilities, no redaction,
// 256MB memory limit, and shared compilation cache.
//
// Example usage:
//
//	// Simple case (defaults)
//	runtime, err := NewRuntime(ctx, version)
//
//	// With capabilities and redaction
//	runtime, err := NewRuntime(ctx, version,
//	    WithCapabilities(caps),
//	    WithRedactor(redactor),
//	    WithMemoryLimit(512),
//	)
//
//	// Test isolation (separate cache)
//	runtime, err := NewRuntime(ctx, version,
//	    WithCompilationCache(wazero.NewCompilationCache()),
//	)
func NewRuntime(ctx context.Context, version build.Info, opts ...RuntimeOption) (*Runtime, error) {
	// Apply options
	cfg := &runtimeConfig{
		memoryLimitMB: 0,                  // 0 = default (256MB)
		cache:         defaultGlobalCache, // Use shared cache by default
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// Determine memory limit
	// 0 = default (256MB)
	// -1 = unlimited
	// >0 = explicit limit in MB
	switch {
	case cfg.memoryLimitMB == 0:
		cfg.memoryLimitMB = 256 // Default: 256MB
		slog.Info("using default WASM memory limit", "mb", cfg.memoryLimitMB)
	case cfg.memoryLimitMB == -1:
		slog.Warn("WASM memory limit disabled (unlimited memory)")
		// Pass to wazero as is (unlimited)
	case cfg.memoryLimitMB > 0:
		if cfg.memoryLimitMB < 64 {
			slog.Warn("WASM memory limit very low, plugins may fail", "mb", cfg.memoryLimitMB)
		}
	default:
		return nil, fmt.Errorf("invalid WASM memory limit: %d (must be -1 or >= 0)", cfg.memoryLimitMB)
	}

	// Ensure the configured memory limit cannot overflow when converted to pages.
	// Each page is 64KB, i.e. 16 pages per MB, so we require memoryLimitMB*16 <= math.MaxUint32.
	// 268435455 = math.MaxUint32 / 16.
	if cfg.memoryLimitMB > 268435455 {
		return nil, fmt.Errorf("invalid WASM memory limit: %d MB (too large)", cfg.memoryLimitMB)
	}

	// Create pure Go WASM runtime with compilation cache.
	config := wazero.NewRuntimeConfig().WithCompilationCache(cfg.cache)

	// Apply memory limit if not unlimited
	if cfg.memoryLimitMB > 0 {
		// Convert MB to pages (1 page = 64KB)
		// 1 MB = 1024 KB = 16 * 64KB
		pages := uint32(cfg.memoryLimitMB * 16) //nolint:gosec // G115: memoryLimitMB is validated to avoid overflow
		config = config.WithMemoryLimitPages(pages)
	}

	r := wazero.NewRuntimeWithConfig(ctx, config)

	// Instantiate WASI for system calls (clock, random, etc.).
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, r); err != nil {
		_ = r.Close(ctx)
		return nil, fmt.Errorf("failed to instantiate WASI: %w", err)
	}

	// Register host functions with capability enforcement.
	if err := hostfuncs.RegisterHostFunctions(ctx, r, version, cfg.caps); err != nil {
		_ = r.Close(ctx)
		return nil, fmt.Errorf("failed to register host functions: %w", err)
	}

	return &Runtime{
		runtime:             r,
		plugins:             make(map[string]*Plugin),
		version:             version,
		redactor:            cfg.redactor,
		grantedCapabilities: cfg.caps,
		frozenEnv:           os.Environ(), // Freeze environment at startup for security
	}, nil
}

// LoadPlugin compiles and caches a plugin, and is safe for concurrent use.
//
// The first call for a given plugin name compiles the provided WASM bytes,
// creates a Plugin, and stores it in an internal cache keyed by name. Subsequent
// calls with the same name return the previously cached Plugin instance; the
// WASM module is not recompiled.
//
// To reduce contention while remaining thread-safe, LoadPlugin uses a
// double-checked locking pattern around the plugin cache: it first checks the
// cache under a read lock, and only acquires a write lock if the plugin is
// not yet present, re-checking the cache under the write lock before compiling.
// Callers do not need to provide additional synchronization when calling
// LoadPlugin from multiple goroutines.
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
	var stdout, stderr io.Writer = os.Stdout, os.Stderr
	if r.redactor != nil {
		// Wrap OS streams with redaction to prevent secret leakage
		stdout = sensitivedata.NewWriter(os.Stdout, r.redactor)
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
		// Plugin not found in the runtime's plugin registry.
		// The caller can treat this as "plugin not found or not loaded".
		return nil, fmt.Errorf("plugin %s not found or not loaded", pluginName)
	}

	// Get the manifest from the plugin
	manifest, err := plugin.Manifest(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get manifest for plugin %s: %w", pluginName, err)
	}

	if len(manifest.ConfigSchema) == 0 {
		// Plugin doesn't provide a schema
		return nil, nil
	}

	return manifest.ConfigSchema, nil
}

// Close closes the runtime and cleans up resources
func (r *Runtime) Close(ctx context.Context) error {
	return r.runtime.Close(ctx)
}
