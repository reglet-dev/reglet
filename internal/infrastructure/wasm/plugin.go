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

	"github.com/reglet-dev/reglet/internal/domain/capabilities"
	"github.com/reglet-dev/reglet/internal/infrastructure/wasm/hostfuncs"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// Plugin manages the lifecycle and execution of a compiled WASM module.
type Plugin struct {
	name    string
	module  wazero.CompiledModule
	runtime wazero.Runtime

	// Mutex protects cached metadata
	mu sync.Mutex

	// Cached plugin info
	info *PluginInfo

	// Cached schema
	schema *ConfigSchema

	// Redacted output streams (wraps os.Stderr with redaction)
	stdout io.Writer
	stderr io.Writer

	// Granted capabilities for this plugin (used to build filesystem mounts)
	capabilities []capabilities.Capability

	// Frozen environment snapshot from runtime initialization (prevents runtime env leakage)
	frozenEnv []string
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
	if path == "/**" || path == "/*" {
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

	// SECURITY FIX: Handle relative paths safely
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

	for _, cap := range p.capabilities {
		if cap.Kind != "fs" {
			continue
		}

		// Parse pattern: "read:/etc/hosts" or "write:/var/log/**"
		parts := strings.SplitN(cap.Pattern, ":", 2)
		if len(parts) != 2 {
			slog.Warn("invalid filesystem capability pattern",
				"plugin", p.name,
				"pattern", cap.Pattern)
			continue
		}

		operation := parts[0] // "read" or "write"
		pattern := parts[1]   // "/etc/hosts" or "/var/log/**"

		// Extract mount path
		mountPath := extractMountPath(pattern)

		// Skip empty mount paths (indicates error in path extraction)
		if mountPath == "" {
			slog.Warn("skipping invalid capability pattern - could not determine safe mount path",
				"plugin", p.name,
				"pattern", cap.Pattern)
			continue
		}

		// Warn about root access
		if mountPath == "/" || pattern == "/**" {
			slog.Warn("plugin granted root filesystem access",
				"plugin", p.name,
				"capability", cap.Pattern)
		}

		// Track mount (don't deduplicate per user preference)
		mountKey := fmt.Sprintf("%s:%s", operation, mountPath)
		if seenPaths[mountKey] {
			continue // Same operation + path already added
		}
		seenPaths[mountKey] = true

		mounts = append(mounts, fsMount{
			hostPath:  mountPath,
			guestPath: mountPath, // Mount at same path in guest
			readOnly:  operation == "read",
		})
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
	if len(p.capabilities) > 0 {
		config = p.injectEnvironmentVariables(config)
	}

	return config
}

// injectEnvironmentVariables filters host environment variables based on granted capabilities
func (p *Plugin) injectEnvironmentVariables(config wazero.ModuleConfig) wazero.ModuleConfig {
	// Get all granted env capabilities for this plugin
	envCapabilities := []capabilities.Capability{}
	for _, cap := range p.capabilities {
		if cap.Kind == "env" {
			envCapabilities = append(envCapabilities, cap)
		}
	}

	if len(envCapabilities) == 0 {
		return config // No env capabilities granted
	}

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

		// Check if this key is allowed by any granted capability
		for _, cap := range envCapabilities {
			if capabilities.MatchEnvironmentPattern(key, cap.Pattern) {
				allowedEnv = append(allowedEnv, envVar)
				slog.Debug("injecting environment variable",
					"plugin", p.name,
					"key", key,
					"capability", cap.String())
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

// createInstance instantiates the WASM module with a fresh memory environment.
// It ensures thread safety by providing isolated memory for each execution.
func (p *Plugin) createInstance(ctx context.Context) (api.Module, error) {
	// Create fresh instance every time - no caching
	instance, err := p.runtime.InstantiateModule(ctx, p.module, p.createModuleConfig(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate plugin %s: %w", p.name, err)
	}

	// Debug: List all exported functions
	// for _, def := range instance.ExportedFunctionDefinitions() {
	// 	fmt.Fprintf(os.Stderr, "DEBUG: Exported function: %s from %s\n", def.Name(), p.name)
	// }

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

// Describe executes the plugin's 'describe' function to retrieve metadata.
func (p *Plugin) Describe(ctx context.Context) (*PluginInfo, error) {
	// Wrap context with plugin name for host functions
	ctx = hostfuncs.WithPluginName(ctx, p.name)

	p.mu.Lock()
	if p.info != nil {
		info := p.info
		p.mu.Unlock()
		return info, nil
	}
	p.mu.Unlock()

	instance, err := p.createInstance(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = instance.Close(ctx)
	}()

	describeFn := instance.ExportedFunction("describe")
	if describeFn == nil {
		return nil, fmt.Errorf("plugin %s does not export describe() function", p.name)
	}

	results, err := describeFn.Call(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to call describe(): %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("describe() returned no results")
	}

	packed := results[0]
	ptr := uint32(packed >> 32)         //nolint:gosec // G115: WASM32 pointers are always 32-bit
	size := uint32(packed & 0xFFFFFFFF) //nolint:gosec // G115: WASM32 lengths are always 32-bit

	if ptr == 0 || size == 0 {
		return nil, fmt.Errorf("describe() returned null pointer or zero length")
	}

	data, err := p.readString(ctx, instance, ptr, size)
	if err != nil {
		return nil, fmt.Errorf("failed to read describe() result: %w", err)
	}

	info, err := parsePluginInfo(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse plugin info: %w", err)
	}

	p.mu.Lock()
	p.info = info
	p.mu.Unlock()

	return info, nil
}

// Schema executes the plugin's 'schema' function to retrieve configuration definitions.
func (p *Plugin) Schema(ctx context.Context) (*ConfigSchema, error) {
	ctx = hostfuncs.WithPluginName(ctx, p.name)

	p.mu.Lock()
	if p.schema != nil {
		schema := p.schema
		p.mu.Unlock()
		return schema, nil
	}
	p.mu.Unlock()

	instance, err := p.createInstance(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = instance.Close(ctx)
	}()

	schemaFn := instance.ExportedFunction("schema")
	if schemaFn == nil {
		return nil, fmt.Errorf("plugin %s does not export schema() function", p.name)
	}

	results, err := schemaFn.Call(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to call schema(): %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("schema() returned no results")
	}

	packed := results[0]
	ptr := uint32(packed >> 32)         //nolint:gosec // G115: WASM32 pointers are always 32-bit
	size := uint32(packed & 0xFFFFFFFF) //nolint:gosec // G115: WASM32 lengths are always 32-bit

	if ptr == 0 || size == 0 {
		return nil, fmt.Errorf("schema() returned null pointer or zero length")
	}

	data, err := p.readString(ctx, instance, ptr, size)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema() result: %w", err)
	}

	// Store raw JSON schema for now.
	schema := &ConfigSchema{
		Fields:    []FieldDef{},
		RawSchema: data,
	}

	p.mu.Lock()
	p.schema = schema
	p.mu.Unlock()

	return schema, nil
}

// Observe executes the main validation logic of the plugin.
func (p *Plugin) Observe(ctx context.Context, cfg Config) (*PluginObservationResult, error) {
	// Wrap context with plugin name so host functions can access it
	ctx = hostfuncs.WithPluginName(ctx, p.name)

	// Create FRESH instance for this call - ensures thread safety
	instance, err := p.createInstance(ctx)
	if err != nil {
		return nil, err
	}
	// CRITICAL: Always close instance when done
	defer func() {
		_ = instance.Close(ctx) // Best-effort cleanup
	}()

	// Get the observe function
	observeFn := instance.ExportedFunction("observe")
	if observeFn == nil {
		return nil, fmt.Errorf("plugin %s does not export observe() function", p.name)
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

	// Parse JSON result directly into internal/wasm/types.Evidence
	var hostEvidence Evidence
	if err := json.Unmarshal(resultData, &hostEvidence); err != nil {
		return nil, fmt.Errorf("failed to parse observe() result into internal/wasm/types.Evidence: %w", err)
	}

	// Construct and return PluginObservationResult
	// Note: Evidence.Error represents application-level errors (validation, lookup failures, etc.)
	// PluginObservationResult.Error represents WASM execution errors (panics, plugin failures)
	// Don't propagate Evidence.Error to PluginObservationResult.Error - they serve different purposes
	return &PluginObservationResult{
			Evidence: &hostEvidence,
			Error:    nil, // Plugin executed successfully, errors are in Evidence
		},
		nil
}

// Close performs any necessary cleanup.
// Currently a no-op as instances are ephemeral.
func (p *Plugin) Close() error {
	// No cached instance to close anymore
	// Each method call creates and closes its own instance
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

// parsePluginInfo decodes the JSON metadata returned by the plugin.
func parsePluginInfo(data []byte) (*PluginInfo, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	info := &PluginInfo{}

	// Parse required fields
	if name, ok := raw["name"].(string); ok {
		info.Name = name
	}

	if version, ok := raw["version"].(string); ok {
		info.Version = version
	}

	if description, ok := raw["description"].(string); ok {
		info.Description = description
	}

	// Parse capabilities array
	if caps, ok := raw["capabilities"].([]interface{}); ok {
		for _, capRaw := range caps {
			if capMap, ok := capRaw.(map[string]interface{}); ok {
				var capability capabilities.Capability
				if kind, ok := capMap["kind"].(string); ok {
					capability.Kind = kind
				}
				if pattern, ok := capMap["pattern"].(string); ok {
					capability.Pattern = pattern
				}
				info.Capabilities = append(info.Capabilities, capability)
			}
		}
	}

	return info, nil
}
