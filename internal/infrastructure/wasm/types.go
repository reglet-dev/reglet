// Package wasm provides WebAssembly runtime infrastructure for Reglet plugins.
// It manages plugin loading, execution, and capability-based sandboxing using wazero.
package wasm

import (
	"github.com/reglet-dev/reglet/internal/domain/capabilities"
	"github.com/reglet-dev/reglet/internal/domain/execution"
)

// PluginInfo contains metadata about a plugin
// Maps to the WIT plugin-info record
type PluginInfo struct {
	Name         string
	Version      string
	Description  string
	Capabilities []capabilities.Capability
}

// Config represents plugin configuration
// Maps to the WIT config record
type Config struct {
	Values map[string]interface{}
}

// Evidence is re-exported from domain for backward compatibility in this package.
// Use execution.Evidence from domain layer.
type Evidence = execution.Evidence

// PluginError is re-exported from domain for backward compatibility in this package.
// Use execution.PluginError from domain layer.
type PluginError = execution.PluginError

// ConfigSchema represents the JSON Schema for plugin configuration
// Maps to the WIT config-schema record
type ConfigSchema struct {
	Fields    []FieldDef
	RawSchema []byte // Raw JSON Schema data from plugin
}

// FieldDef represents a configuration field definition
// Maps to the WIT field-def record
type FieldDef struct {
	Name        string
	FieldType   string
	Description string
	Required    bool
}

// PluginObservationResult is the result of running an observation through a WASM plugin.
// This is a low-level boundary type.
type PluginObservationResult struct {
	Evidence *execution.Evidence
	Error    *execution.PluginError
}
