package wasm

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/execution"
	"github.com/reglet-dev/reglet/internal/infrastructure/wasm/hostfuncs"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// Plugin manages the lifecycle and execution of a compiled WASM module.
type Plugin struct {
	runtime      wazero.Runtime
	stdout       io.Writer
	stderr       io.Writer
	module       wazero.CompiledModule
	moduleConfig wazero.ModuleConfig
	manifest     *entities.Manifest
	instancePool chan api.Module

	capabilities *entities.GrantSet
	name         string
	frozenEnv    []string
	poolSize     int
	configOnce   sync.Once
	mu           sync.Mutex
}

// fsMount represents a filesystem mount configuration
type fsMount struct {
	hostPath  string
	guestPath string
	readOnly  bool
}

// Name returns the unique identifier of the plugin.
func (p *Plugin) Name() string {
	return p.name
}

// extractMountPath returns the directory to mount for a filesystem pattern.
// For files: returns parent directory (e.g., "/etc/ssh/sshd_config" → "/etc/ssh")
// For directories: returns the directory itself (e.g., "/var/log/**" → "/var/log")
func extractMountPath(pattern string) string {
	// Remove operation prefix (e.g., "read:" or "write:")
	parts := strings.SplitN(pattern, ":", 2)
	path := pattern
	if len(parts) == 2 {
		path = parts[1]
	}

	// Handle root wildcard pattern first (before trimming)
	// These patterns mean "all files" and should mount the root filesystem
	if path == "/**" || path == "/*" || path == "**" || path == "*" {
		return "/"
	}

	// Handle wildcard patterns - these are directory patterns
	if strings.HasSuffix(path, "/**") {
		// "/var/log/**" → "/var/log"
		return strings.TrimSuffix(path, "/**")
	}
	if strings.HasSuffix(path, "/*") {
		// "/var/log/*" → "/var/log"
		return strings.TrimSuffix(path, "/*")
	}

	// Handle root pattern
	if path == "/" {
		return "/"
	}

	// For non-wildcard patterns, assume it's a file and return parent directory
	// This handles cases like "/etc/hosts" → "/etc"
	dir := filepath.Dir(path)

	// Handle relative paths safely
	// CRITICAL: Never mount host root (/) for relative paths!
	if dir == "." {
		// Relative path detected - mount current working directory, NOT root
		cwd, err := os.Getwd()
		if err != nil {
			// If we can't determine CWD, log error and return empty string
			// Empty string will be caught and skipped by extractFilesystemMounts
			slog.Error("cannot determine current working directory for relative path capability",
				"pattern", pattern,
				"error", err)
			return "" // Signal to skip this mount
		}
		slog.Warn("relative path in capability - mounting current working directory",
			"pattern", pattern,
			"mount_path", cwd)
		return cwd
	}

	return dir
}

// extractFilesystemMounts builds mount configurations from granted filesystem capabilities.
func (p *Plugin) extractFilesystemMounts() []fsMount {
	var mounts []fsMount
	seenPaths := make(map[string]bool)

	if p.capabilities == nil || p.capabilities.FS == nil {
		return mounts
	}

	for _, rule := range p.capabilities.FS.Rules {
		// Process read paths
		for _, pattern := range rule.Read {
			mountPath := extractMountPath(pattern)
			if mountPath == "" {
				slog.Warn("skipping invalid capability pattern - could not determine safe mount path",
					"plugin", p.name,
					"pattern", pattern)
				continue
			}

			if mountPath == "/" || pattern == "/**" {
				slog.Warn("plugin granted root filesystem access",
					"plugin", p.name,
					"capability", "read:"+pattern)
			}

			mountKey := fmt.Sprintf("read:%s", mountPath)
			if seenPaths[mountKey] {
				continue
			}
			seenPaths[mountKey] = true

			mounts = append(mounts, fsMount{
				hostPath:  mountPath,
				guestPath: mountPath,
				readOnly:  true,
			})
		}

		// Process write paths
		for _, pattern := range rule.Write {
			mountPath := extractMountPath(pattern)
			if mountPath == "" {
				slog.Warn("skipping invalid capability pattern - could not determine safe mount path",
					"plugin", p.name,
					"pattern", pattern)
				continue
			}

			if mountPath == "/" || pattern == "/**" {
				slog.Warn("plugin granted root filesystem access",
					"plugin", p.name,
					"capability", "write:"+pattern)
			}

			mountKey := fmt.Sprintf("write:%s", mountPath)
			if seenPaths[mountKey] {
				continue
			}
			seenPaths[mountKey] = true

			mounts = append(mounts, fsMount{
				hostPath:  mountPath,
				guestPath: mountPath,
				readOnly:  false,
			})
		}
	}

	return mounts
}

// createModuleConfig builds the wazero module configuration with necessary host functions.
// It enables filesystem access, time, random, and logging.
// stdout/stderr are automatically redacted to prevent secret leakage to logs.
func (p *Plugin) createModuleConfig(_ context.Context) wazero.ModuleConfig {
	// Build filesystem mounts from capabilities
	mounts := p.extractFilesystemMounts()
	fsConfig := wazero.NewFSConfig()

	for _, mount := range mounts {
		if mount.readOnly {
			fsConfig = fsConfig.WithReadOnlyDirMount(mount.hostPath, mount.guestPath)
			slog.Debug("mounting read-only filesystem",
				"plugin", p.name,
				"path", mount.hostPath)
		} else {
			fsConfig = fsConfig.WithDirMount(mount.hostPath, mount.guestPath)
			slog.Debug("mounting read-write filesystem",
				"plugin", p.name,
				"path", mount.hostPath)
		}
	}

	// Log when plugin has no filesystem access
	if len(mounts) == 0 {
		slog.Debug("plugin has no filesystem access",
			"plugin", p.name)
	}

	config := wazero.NewModuleConfig().
		WithFSConfig(fsConfig). // Now uses capability-driven mounts
		WithSysWalltime().
		WithSysNanotime().
		WithSysNanosleep().
		WithRandSource(rand.Reader).
		// SECURITY: Use redacted writers to prevent secrets from leaking to logs
		WithStderr(p.stderr).
		WithStdout(p.stdout)

	// Inject environment variables based on granted capabilities
	if p.capabilities != nil && p.capabilities.Env != nil && len(p.capabilities.Env.Variables) > 0 {
		config = p.injectEnvironmentVariables(config)
	}

	return config
}

// injectEnvironmentVariables filters host environment variables based on granted capabilities
func (p *Plugin) injectEnvironmentVariables(config wazero.ModuleConfig) wazero.ModuleConfig {
	if p.capabilities == nil || p.capabilities.Env == nil || len(p.capabilities.Env.Variables) == 0 {
		return config // No env capabilities granted
	}

	envPatterns := p.capabilities.Env.Variables

	// Use frozen environment snapshot from runtime initialization
	// This prevents runtime environment changes from leaking to plugins
	hostEnv := p.frozenEnv

	// Filter environment variables that match granted patterns
	allowedEnv := []string{}
	for _, envVar := range hostEnv {
		// Parse "KEY=VALUE"
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]

		// Check if this key is allowed by any granted pattern
		for _, pattern := range envPatterns {
			if matchEnvironmentPattern(key, pattern) {
				allowedEnv = append(allowedEnv, envVar)
				slog.Debug("injecting environment variable",
					"plugin", p.name,
					"key", key,
					"pattern", pattern)
				break
			}
		}
	}

	// Inject allowed variables
	for _, envVar := range allowedEnv {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) == 2 {
			config = config.WithEnv(parts[0], parts[1])
		}
	}

	return config
}

// matchEnvironmentPattern checks if an environment variable key matches a capability pattern.
// Supports exact match ("AWS_REGION"), prefix match ("AWS_*"), and wildcard ("*").
func matchEnvironmentPattern(key, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(key, prefix)
	}
	return key == pattern
}

// defaultPoolSize is the number of pre-instantiated WASM instances to keep ready.
// This significantly speeds up concurrent Observe() calls by avoiding instantiation overhead.
const defaultPoolSize = 16

// initPool initializes the instance pool lazily on first use.
func (p *Plugin) initPool(ctx context.Context) {
	p.configOnce.Do(func() {
		p.moduleConfig = p.createModuleConfig(ctx)
		p.poolSize = defaultPoolSize
		p.instancePool = make(chan api.Module, p.poolSize)
	})
}

// WarmPool pre-creates instances in the pool for faster concurrent access.
// Instances are created in parallel to minimize warmup time.
// Call this before heavy concurrent usage to avoid instantiation delays.
// Returns the number of instances created.
func (p *Plugin) WarmPool(ctx context.Context, count int) (int, error) {
	p.initPool(ctx)

	// Cap count to pool size
	if count > p.poolSize {
		count = p.poolSize
	}

	// Create instances in parallel for faster warmup
	type result struct {
		instance api.Module
		err      error
	}
	results := make(chan result, count)

	for i := 0; i < count; i++ {
		go func() {
			instance, err := p.newInstance(ctx)
			results <- result{instance, err}
		}()
	}

	// Collect results
	created := 0
	var firstErr error
	for i := 0; i < count; i++ {
		r := <-results
		if r.err != nil {
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}

		// Try to add to pool
		select {
		case p.instancePool <- r.instance:
			created++
		default:
			// Pool is full, close this instance
			_ = r.instance.Close(ctx)
		}
	}

	return created, firstErr
}

// acquireInstance gets an instance from the pool or creates a new one.
// Callers MUST call releaseInstance when done.
func (p *Plugin) acquireInstance(ctx context.Context) (api.Module, error) {
	p.initPool(ctx)

	// Try to get from pool (non-blocking)
	select {
	case instance := <-p.instancePool:
		return instance, nil
	default:
		// Pool empty, create new instance
		return p.newInstance(ctx)
	}
}

// releaseInstance returns an instance to the pool for reuse.
// If the pool is full, the instance is closed.
func (p *Plugin) releaseInstance(ctx context.Context, instance api.Module) {
	if instance == nil {
		return
	}

	// Try to return to pool (non-blocking)
	select {
	case p.instancePool <- instance:
		// Successfully returned to pool
	default:
		// Pool full, close instance
		_ = instance.Close(ctx)
	}
}

// newInstance creates a fresh WASM module instance.
func (p *Plugin) newInstance(ctx context.Context) (api.Module, error) {
	instance, err := p.runtime.InstantiateModule(ctx, p.module, p.moduleConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate plugin %s: %w", p.name, err)
	}

	// Call _initialize for WASI modules built with -buildmode=c-shared
	// This must be called before any other exported functions
	initFn := instance.ExportedFunction("_initialize")
	if initFn != nil {
		if _, err := initFn.Call(ctx); err != nil {
			_ = instance.Close(ctx) // Best-effort cleanup
			return nil, fmt.Errorf("failed to initialize plugin %s: %w", p.name, err)
		}
	}

	return instance, nil
}

// Manifest retrieves the plugin's metadata including capabilities and config schema.
func (p *Plugin) Manifest(ctx context.Context) (*entities.Manifest, error) {
	// Wrap context with plugin name for host functions
	ctx = hostfuncs.WithPluginName(ctx, p.name)

	p.mu.Lock()
	if p.manifest != nil {
		manifest := p.manifest
		p.mu.Unlock()
		return manifest, nil
	}
	p.mu.Unlock()

	instance, err := p.acquireInstance(ctx)
	if err != nil {
		return nil, err
	}
	defer p.releaseInstance(ctx, instance)

	manifestFn := instance.ExportedFunction("_manifest")
	if manifestFn == nil {
		return nil, fmt.Errorf("plugin %s does not export _manifest() function", p.name)
	}

	results, err := manifestFn.Call(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to call _manifest(): %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("_manifest() returned no results")
	}

	packed := results[0]
	ptr := uint32(packed >> 32)         //nolint:gosec // G115: WASM32 pointers are always 32-bit
	size := uint32(packed & 0xFFFFFFFF) //nolint:gosec // G115: WASM32 lengths are always 32-bit

	if ptr == 0 || size == 0 {
		return nil, fmt.Errorf("_manifest() returned null pointer or zero length")
	}

	data, err := p.readString(ctx, instance, ptr, size)
	if err != nil {
		return nil, fmt.Errorf("failed to read _manifest() result: %w", err)
	}

	manifest := &entities.Manifest{}
	if err := json.Unmarshal(data, manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest JSON: %w", err)
	}

	// Extract capabilities into GrantSet for runtime configuration
	// Manifest now directly contains GrantSet (no conversion needed)

	p.mu.Lock()
	p.manifest = manifest
	p.capabilities = &manifest.Capabilities
	p.mu.Unlock()

	return manifest, nil
}

// Observe executes the main validation logic of the plugin.
func (p *Plugin) Observe(ctx context.Context, cfg Config) (*PluginObservationResult, error) {
	// Wrap context with plugin name so host functions can access it
	ctx = hostfuncs.WithPluginName(ctx, p.name)

	// Acquire instance from pool (or create new one)
	instance, err := p.acquireInstance(ctx)
	if err != nil {
		return nil, err
	}
	// Return instance to pool when done (or close if pool is full)
	defer p.releaseInstance(ctx, instance)

	// Get the observe function
	observeFn := instance.ExportedFunction("_observe")
	if observeFn == nil {
		return nil, fmt.Errorf("plugin %s does not export _observe() function", p.name)
	}

	// Marshal config to JSON
	configData, err := json.Marshal(cfg.Values)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write config to WASM memory
	configPtr, err := p.writeToMemory(ctx, instance, configData)
	if err != nil {
		return nil, fmt.Errorf("failed to write config to WASM memory: %w", err)
	}

	// CRITICAL: Ensure config memory is always deallocated, even on error
	defer func() {
		// Prevent cleanup panic from clobbering an existing panic
		defer func() {
			_ = recover()
		}()

		deallocateFn := instance.ExportedFunction("deallocate")
		if deallocateFn != nil {
			//nolint:errcheck,gosec // G104: Deallocation is best-effort cleanup
			deallocateFn.Call(ctx, uint64(configPtr), uint64(len(configData)))
		}
	}()

	// Call observe(configPtr, configLen)
	results, err := observeFn.Call(ctx, uint64(configPtr), uint64(len(configData)))
	if err != nil {
		return nil, fmt.Errorf("failed to call observe(): %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("observe() returned no results")
	}

	// Unpack ptr and length from uint64
	packed := results[0]
	resultPtr := uint32(packed >> 32)         //nolint:gosec // G115: WASM32 pointers are always 32-bit
	resultSize := uint32(packed & 0xFFFFFFFF) //nolint:gosec // G115: WASM32 lengths are always 32-bit

	if resultPtr == 0 || resultSize == 0 {
		return nil, fmt.Errorf("observe() returned null pointer or zero length")
	}

	// Read EXACT size
	resultData, err := p.readString(ctx, instance, resultPtr, resultSize)
	if err != nil {
		return nil, fmt.Errorf("failed to read observe() result: %w", err)
	}

	// SDK Result Adaptation:
	// The SDK returns a Result with a string Status ("success", "failure", "error"),
	// but the Host's Evidence struct expects a boolean Status.
	// We unmarshal into an intermediate struct and adapt it.
	type sdkErrorDetail struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	type sdkResult struct {
		Timestamp time.Time              `json:"timestamp"`
		Data      map[string]interface{} `json:"data"`
		Error     *sdkErrorDetail        `json:"error"`
		Status    string                 `json:"status"`
		Message   string                 `json:"message"`
	}

	var result sdkResult
	if err := json.Unmarshal(resultData, &result); err != nil {
		return nil, fmt.Errorf("failed to parse observe() result into sdkResult: %w", err)
	}

	// strict mapping: Only "success" is true. "failure" and "error" are false.
	statusBool := (result.Status == "success")

	var pluginErr *execution.PluginError
	switch {
	case result.Error != nil:
		pluginErr = &execution.PluginError{
			Code:    result.Error.Code,
			Message: result.Error.Message,
		}
	case result.Status == "error":
		// Fallback if error status but no error detail (shouldn't happen with valid SDK)
		pluginErr = &execution.PluginError{
			Code:    "UNKNOWN_ERROR",
			Message: result.Message,
		}
	}

	hostEvidence := Evidence{
		Timestamp: result.Timestamp,
		Data:      result.Data,
		Error:     pluginErr,
		Status:    statusBool,
	}

	// Construct and return PluginObservationResult
	// Note: Evidence.Error represents application-level errors (validation, lookup failures, etc.)
	// PluginObservationResult.Error represents WASM execution errors (panics, plugin failures)
	// Don't propagate Evidence.Error to PluginObservationResult.Error - they serve different purposes
	return &PluginObservationResult{
		Evidence: &hostEvidence,
		Error:    nil, // Plugin executed successfully, errors are in Evidence
	}, nil
}

// Close performs any necessary cleanup including draining the instance pool.
func (p *Plugin) Close() error {
	// Drain the instance pool
	if p.instancePool != nil {
		close(p.instancePool)
		for instance := range p.instancePool {
			_ = instance.Close(context.Background())
		}
	}
	return nil
}

// readString safely reads a byte slice from WASM memory and deallocates it.
func (p *Plugin) readString(ctx context.Context, instance api.Module, ptr uint32, size uint32) ([]byte, error) {
	// CRITICAL: Ensure memory is always deallocated, even on error
	defer func() {
		// Prevent cleanup panic from clobbering an existing panic
		defer func() {
			_ = recover()
		}()

		deallocateFn := instance.ExportedFunction("deallocate")
		if deallocateFn != nil {
			//nolint:errcheck,gosec // G104: Deallocation is best-effort cleanup
			deallocateFn.Call(ctx, uint64(ptr), uint64(size))
		}
	}()

	// Read EXACT size (no more guessing!)
	data, ok := instance.Memory().Read(ptr, size)
	if !ok {
		return nil, fmt.Errorf("failed to read memory at offset %d", ptr)
	}

	// Copy to our own buffer
	result := make([]byte, size)
	copy(result, data)

	return result, nil
}

// writeToMemory allocates WASM memory and copies data into it.
// It returns the pointer to the allocated block.
func (p *Plugin) writeToMemory(ctx context.Context, instance api.Module, data []byte) (uint32, error) {
	// Get the allocate function from the plugin
	allocateFn := instance.ExportedFunction("allocate")
	if allocateFn == nil {
		return 0, fmt.Errorf("plugin does not export allocate() function")
	}

	// Allocate memory for the data
	results, err := allocateFn.Call(ctx, uint64(len(data)))
	if err != nil {
		return 0, fmt.Errorf("failed to allocate memory: %w", err)
	}

	if len(results) == 0 {
		return 0, fmt.Errorf("allocate() returned no results")
	}

	ptr := uint32(results[0]) //nolint:gosec // G115: WASM32 pointers are always 32-bit
	if ptr == 0 {
		return 0, fmt.Errorf("allocate() returned null pointer")
	}

	// Write data to the allocated memory
	if !instance.Memory().Write(ptr, data) {
		return 0, fmt.Errorf("failed to write to WASM memory at offset %d", ptr)
	}

	// Debug: Verify the write by reading it back
	// readBack, ok := instance.Memory().Read(ptr, uint32(len(data)))
	// if !ok {
	// 	return 0, fmt.Errorf("failed to read back written data at offset %d", ptr)
	// }
	// fmt.Printf("DEBUG writeToMemory: Wrote %d bytes to ptr %d. Readback hex: %% x\n", len(data), ptr, readBack)

	return ptr, nil
}
