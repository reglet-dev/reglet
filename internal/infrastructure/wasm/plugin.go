package wasm

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/whiskeyjimbo/reglet/internal/domain/capabilities"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/wasm/hostfuncs"
)

//nolint:gosec // G115: uint64->uint32 conversions are safe for WASM32 address space

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
}

// Name returns the unique identifier of the plugin.
func (p *Plugin) Name() string {
	return p.name
}

// createModuleConfig builds the wazero module configuration with necessary host functions.
// It enables filesystem access, time, random, and logging.
func (p *Plugin) createModuleConfig() wazero.ModuleConfig {
	return wazero.NewModuleConfig().
		// Mount root filesystem to allow access to system files (e.g. /etc/ssh/sshd_config).
		// Note: Phase 2 will introduce finer-grained capability enforcement for WASI.
		WithFSConfig(wazero.NewFSConfig().WithDirMount("/", "/")).
		WithSysWalltime().
		WithSysNanotime().
		WithSysNanosleep().
		WithRandSource(rand.Reader).
		WithStderr(os.Stderr).
		WithStdout(os.Stderr)
}

// createInstance instantiates the WASM module with a fresh memory environment.
// It ensures thread safety by providing isolated memory for each execution.
func (p *Plugin) createInstance(ctx context.Context) (api.Module, error) {
	// Create fresh instance every time - no caching
	instance, err := p.runtime.InstantiateModule(ctx, p.module, p.createModuleConfig())
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
	ptr := uint32(packed >> 32)
	size := uint32(packed & 0xFFFFFFFF)

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
	ptr := uint32(packed >> 32)
	size := uint32(packed & 0xFFFFFFFF)

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
func (p *Plugin) Observe(ctx context.Context, cfg Config) (*ObservationResult, error) {
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
		deallocateFn := instance.ExportedFunction("deallocate")
		if deallocateFn != nil {
			//nolint:errcheck // Deallocation is best-effort cleanup
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
	resultPtr := uint32(packed >> 32)
	resultSize := uint32(packed & 0xFFFFFFFF)

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

	// Construct and return ObservationResult
	// Note: Evidence.Error represents application-level errors (validation, lookup failures, etc.)
	// ObservationResult.Error represents WASM execution errors (panics, plugin failures)
	// Don't propagate Evidence.Error to ObservationResult.Error - they serve different purposes
	return &ObservationResult{
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
		deallocateFn := instance.ExportedFunction("deallocate")
		if deallocateFn != nil {
			//nolint:errcheck // Deallocation is best-effort cleanup
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

	ptr := uint32(results[0])
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
