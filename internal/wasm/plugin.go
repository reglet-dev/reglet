package wasm

import (
	"context"
	"fmt"

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

	// Instantiate the module
	// Note: In a real implementation, we would set up host functions here
	// for capability enforcement (fs access, network, etc.)
	instance, err := p.runtime.InstantiateModule(p.ctx, p.module, wazero.NewModuleConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate plugin %s: %w", p.name, err)
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

	// TODO: Call the function and parse the result
	// For now, this is a placeholder - we need to implement the WIT bindings
	// to properly marshal/unmarshal data between Go and WASM

	// Placeholder implementation
	info := &PluginInfo{
		Name:        p.name,
		Version:     "0.0.0",
		Description: "TODO: Parse from WASM",
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

	// TODO: Call the function and parse the result
	// Placeholder implementation
	schema := &ConfigSchema{
		Fields: []FieldDef{},
	}

	p.schema = schema
	return schema, nil
}

// Observe calls the plugin's observe() function with the given config
func (p *Plugin) Observe(_ Config) (*ObservationResult, error) {
	instance, err := p.getInstance()
	if err != nil {
		return nil, err
	}

	// Get the observe function
	observeFn := instance.ExportedFunction("observe")
	if observeFn == nil {
		return nil, fmt.Errorf("plugin %s does not export observe() function", p.name)
	}

	// TODO: Implement proper WIT binding to:
	// 1. Marshal Config to WASM memory
	// 2. Call observe()
	// 3. Unmarshal result (Evidence or Error) from WASM memory

	// Placeholder implementation
	return &ObservationResult{
		Evidence: &Evidence{
			Data: make(map[string]interface{}),
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
