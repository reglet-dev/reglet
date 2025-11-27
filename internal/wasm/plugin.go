package wasm

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// Plugin represents a loaded WASM plugin
type Plugin struct {
	name    string
	module  wazero.CompiledModule
	runtime wazero.Runtime
	ctx     context.Context

	// Cached plugin info from describe() (thread-safe, immutable after first read)
	info *PluginInfo

	// Cached schema from schema() (thread-safe, immutable after first read)
	schema *ConfigSchema
}

// Name returns the plugin name
func (p *Plugin) Name() string {
	return p.name
}

// createModuleConfig creates a fresh module configuration
// Includes stdout/stderr for debugging visibility
func (p *Plugin) createModuleConfig() wazero.ModuleConfig {
	return wazero.NewModuleConfig().
		// Mount host root "/" to guest root "/" for filesystem access
		// TODO Phase 2: Implement proper capability-based restrictions
		WithFSConfig(wazero.NewFSConfig().WithDirMount("/", "/")).
		// Enable time-related syscalls (needed for file timestamps)
		WithSysWalltime().
		WithSysNanotime().
		WithSysNanosleep().
		// Enable random number generation
		WithRandSource(rand.Reader).
		// FIX: Enable logging visibility for debugging
		WithStderr(os.Stderr).
		WithStdout(os.Stderr)
}

// createInstance creates a fresh module instance
// Each call gets isolated WASM memory - thread-safe
func (p *Plugin) createInstance() (api.Module, error) {
	// Create fresh instance every time - no caching
	instance, err := p.runtime.InstantiateModule(p.ctx, p.module, p.createModuleConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate plugin %s: %w", p.name, err)
	}

	// Call _initialize for WASI modules built with -buildmode=c-shared
	// This must be called before any other exported functions
	initFn := instance.ExportedFunction("_initialize")
	if initFn != nil {
		if _, err := initFn.Call(p.ctx); err != nil {
			instance.Close(p.ctx)
			return nil, fmt.Errorf("failed to initialize plugin %s: %w", p.name, err)
		}
	}

	return instance, nil
}

// Describe calls the plugin's describe() function and returns metadata
func (p *Plugin) Describe() (*PluginInfo, error) {
	// Return cached info if available (thread-safe)
	if p.info != nil {
		return p.info, nil
	}

	// Create fresh instance for this call
	instance, err := p.createInstance()
	if err != nil {
		return nil, err
	}
	// CRITICAL: Always close instance when done
	defer instance.Close(p.ctx)

	// Get the describe function
	describeFn := instance.ExportedFunction("describe")
	if describeFn == nil {
		return nil, fmt.Errorf("plugin %s does not export describe() function", p.name)
	}

	// Call describe() - returns packed uint64
	results, err := describeFn.Call(p.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to call describe(): %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("describe() returned no results")
	}

	// FIX: Unpack ptr and length from uint64
	packed := results[0]
	ptr := uint32(packed >> 32)             // High 32 bits
	size := uint32(packed & 0xFFFFFFFF)     // Low 32 bits

	if ptr == 0 || size == 0 {
		return nil, fmt.Errorf("describe() returned null pointer or zero length")
	}

	// Read EXACT size from memory
	data, err := p.readString(instance, ptr, size)
	if err != nil {
		return nil, fmt.Errorf("failed to read describe() result: %w", err)
	}

	// Parse JSON into PluginInfo
	info, err := parsePluginInfo(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse plugin info: %w", err)
	}

	p.info = info
	return info, nil
}

// Schema calls the plugin's schema() function and returns the config schema
func (p *Plugin) Schema() (*ConfigSchema, error) {
	// Return cached schema if available (thread-safe)
	if p.schema != nil {
		return p.schema, nil
	}

	// Create fresh instance for this call
	instance, err := p.createInstance()
	if err != nil {
		return nil, err
	}
	// CRITICAL: Always close instance when done
	defer instance.Close(p.ctx)

	// Get the schema function
	schemaFn := instance.ExportedFunction("schema")
	if schemaFn == nil {
		return nil, fmt.Errorf("plugin %s does not export schema() function", p.name)
	}

	// Call schema() - returns packed uint64
	results, err := schemaFn.Call(p.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to call schema(): %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("schema() returned no results")
	}

	// FIX: Unpack ptr and length from uint64
	packed := results[0]
	ptr := uint32(packed >> 32)
	size := uint32(packed & 0xFFFFFFFF)

	if ptr == 0 || size == 0 {
		return nil, fmt.Errorf("schema() returned null pointer or zero length")
	}

	// Read exact size
	data, err := p.readString(instance, ptr, size)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema() result: %w", err)
	}

	// Parse JSON into ConfigSchema
	// The plugin returns a JSON Schema object, which we'll store as-is for now
	// In a real implementation, we might want to parse it into a more structured format
	schema := &ConfigSchema{
		Fields: []FieldDef{},
		// Store raw JSON schema for now
		RawSchema: data,
	}

	p.schema = schema
	return schema, nil
}

// Observe calls the plugin's observe() function with the given config
func (p *Plugin) Observe(cfg Config) (*ObservationResult, error) {
	// Create FRESH instance for this call - ensures thread safety
	instance, err := p.createInstance()
	if err != nil {
		return nil, err
	}
	// CRITICAL: Always close instance when done
	defer instance.Close(p.ctx)

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
	configPtr, err := p.writeToMemory(instance, configData)
	if err != nil {
		return nil, fmt.Errorf("failed to write config to WASM memory: %w", err)
	}

	// CRITICAL: Ensure config memory is always deallocated, even on error
	defer func() {
		deallocateFn := instance.ExportedFunction("deallocate")
		if deallocateFn != nil {
			deallocateFn.Call(p.ctx, uint64(configPtr), uint64(len(configData)))
		}
	}()

	// Call observe(configPtr, configLen)
	results, err := observeFn.Call(p.ctx, uint64(configPtr), uint64(len(configData)))
	if err != nil {
		return nil, fmt.Errorf("failed to call observe(): %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("observe() returned no results")
	}

	// FIX: Unpack ptr and length from uint64
	packed := results[0]
	resultPtr := uint32(packed >> 32)
	resultSize := uint32(packed & 0xFFFFFFFF)

	if resultPtr == 0 || resultSize == 0 {
		return nil, fmt.Errorf("observe() returned null pointer or zero length")
	}

	// Read EXACT size
	resultData, err := p.readString(instance, resultPtr, resultSize)
	if err != nil {
		return nil, fmt.Errorf("failed to read observe() result: %w", err)
	}

	// Parse JSON result
	var rawResult map[string]interface{}
	if err := json.Unmarshal(resultData, &rawResult); err != nil {
		return nil, fmt.Errorf("failed to parse observe() result: %w", err)
	}

	// Check if result contains an error WITHOUT a status field
	// If there's a status field, it's a valid observation result (even if it includes an error message)
	// If there's ONLY an error field, it's a plugin execution error
	_, hasStatus := rawResult["status"]
	errMsg, hasError := rawResult["error"].(string)

	if hasError && !hasStatus {
		// This is a plugin execution error (e.g., invalid config, marshaling failure)
		return &ObservationResult{
			Error: &PluginError{
				Code:    "plugin_error",
				Message: errMsg,
			},
		}, nil
	}

	// Parse as evidence (includes results with status=false and error messages)
	return &ObservationResult{
		Evidence: &Evidence{
			Data: rawResult,
		},
	}, nil
}

// Close closes the plugin and frees resources
// No-op since instances are created and closed per call
func (p *Plugin) Close() error {
	// No cached instance to close anymore
	// Each method call creates and closes its own instance
	return nil
}

// readString reads exactly 'size' bytes from WASM memory at ptr
// and calls deallocate to free the memory after reading
func (p *Plugin) readString(instance api.Module, ptr uint32, size uint32) ([]byte, error) {
	// CRITICAL: Ensure memory is always deallocated, even on error
	defer func() {
		deallocateFn := instance.ExportedFunction("deallocate")
		if deallocateFn != nil {
			// NOW we know the exact size!
			deallocateFn.Call(p.ctx, uint64(ptr), uint64(size))
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

// writeToMemory allocates memory in the WASM module and writes data to it
// Returns the pointer to the allocated memory
func (p *Plugin) writeToMemory(instance api.Module, data []byte) (uint32, error) {
	// Get the allocate function from the plugin
	allocateFn := instance.ExportedFunction("allocate")
	if allocateFn == nil {
		return 0, fmt.Errorf("plugin does not export allocate() function")
	}

	// Allocate memory for the data
	results, err := allocateFn.Call(p.ctx, uint64(len(data)))
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
	// fmt.Printf("DEBUG writeToMemory: Wrote %d bytes to ptr %d. Readback hex: % x\n", len(data), ptr, readBack)

	return ptr, nil
}

// parsePluginInfo parses JSON data into a PluginInfo struct
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
				cap := Capability{}
				if kind, ok := capMap["kind"].(string); ok {
					cap.Kind = kind
				}
				if pattern, ok := capMap["pattern"].(string); ok {
					cap.Pattern = pattern
				}
				info.Capabilities = append(info.Capabilities, cap)
			}
		}
	}

	return info, nil
}
