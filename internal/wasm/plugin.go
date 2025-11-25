package wasm

import (
	"context"
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

	// Cached instance for reuse
	instance api.Module

	// Cached plugin info from describe()
	info *PluginInfo

	// Cached schema from schema()
	schema *ConfigSchema
}

// Name returns the plugin name
func (p *Plugin) Name() string {
	return p.name
}

// getInstance gets or creates a module instance
func (p *Plugin) getInstance() (api.Module, error) {
	if p.instance != nil {
		return p.instance, nil
	}

	// Configure WASI with filesystem access
	// TODO: Implement proper capability-based restrictions
	// For now, grant full filesystem access for testing
	config := wazero.NewModuleConfig().
		WithFS(os.DirFS("/"))

	// Instantiate the module with WASI filesystem access
	instance, err := p.runtime.InstantiateModule(p.ctx, p.module, config)
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

	p.instance = instance
	return instance, nil
}

// Describe calls the plugin's describe() function and returns metadata
func (p *Plugin) Describe() (*PluginInfo, error) {
	// Return cached info if available
	if p.info != nil {
		return p.info, nil
	}

	instance, err := p.getInstance()
	if err != nil {
		return nil, err
	}

	// Get the describe function
	describeFn := instance.ExportedFunction("describe")
	if describeFn == nil {
		return nil, fmt.Errorf("plugin %s does not export describe() function", p.name)
	}

	// Call describe() - returns a pointer to JSON data in WASM memory
	results, err := describeFn.Call(p.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to call describe(): %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("describe() returned no results")
	}

	ptr := uint32(results[0])
	if ptr == 0 {
		return nil, fmt.Errorf("describe() returned null pointer")
	}

	// Read JSON data from WASM memory
	data, err := p.readString(instance, ptr)
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
	// Return cached schema if available
	if p.schema != nil {
		return p.schema, nil
	}

	instance, err := p.getInstance()
	if err != nil {
		return nil, err
	}

	// Get the schema function
	schemaFn := instance.ExportedFunction("schema")
	if schemaFn == nil {
		return nil, fmt.Errorf("plugin %s does not export schema() function", p.name)
	}

	// Call schema() - returns a pointer to JSON data in WASM memory
	results, err := schemaFn.Call(p.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to call schema(): %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("schema() returned no results")
	}

	ptr := uint32(results[0])
	if ptr == 0 {
		return nil, fmt.Errorf("schema() returned null pointer")
	}

	// Read JSON data from WASM memory
	data, err := p.readString(instance, ptr)
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
	instance, err := p.getInstance()
	if err != nil {
		return nil, err
	}

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

	// Call observe(configPtr, configLen)
	results, err := observeFn.Call(p.ctx, uint64(configPtr), uint64(len(configData)))
	if err != nil {
		return nil, fmt.Errorf("failed to call observe(): %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("observe() returned no results")
	}

	resultPtr := uint32(results[0])
	if resultPtr == 0 {
		return nil, fmt.Errorf("observe() returned null pointer")
	}

	// Read result from WASM memory
	resultData, err := p.readString(instance, resultPtr)
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

// Close closes the plugin instance and frees resources
func (p *Plugin) Close() error {
	if p.instance != nil {
		return p.instance.Close(p.ctx)
	}
	return nil
}

// readString reads a null-terminated string from WASM memory starting at ptr
// For now, we read a fixed size and look for JSON structure
func (p *Plugin) readString(instance api.Module, ptr uint32) ([]byte, error) {
	// Read up to 64KB (reasonable limit for plugin metadata)
	maxSize := uint32(64 * 1024)

	data, ok := instance.Memory().Read(ptr, maxSize)
	if !ok {
		return nil, fmt.Errorf("failed to read memory at offset %d", ptr)
	}

	// Find the end of the JSON data (look for null terminator or parse until valid JSON)
	// For now, find the null terminator
	end := 0
	for i, b := range data {
		if b == 0 {
			end = i
			break
		}
	}

	if end == 0 {
		// No null terminator found in first maxSize bytes, try to use all data
		return data, nil
	}

	return data[:end], nil
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
