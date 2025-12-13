// Package wasm provides WebAssembly runtime infrastructure for Reglet plugins.
// It manages plugin loading, execution, and capability-based sandboxing using wazero.
package wasm

import (
	"time"

	"github.com/whiskeyjimbo/reglet/internal/domain/capabilities"
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

// Evidence represents observation results
// Maps to the WIT evidence record
type Evidence struct {
	Status    bool
	Error     *PluginError // Use PluginError defined in this package
	Timestamp time.Time
	Data      map[string]interface{}
	Raw       *string // Optional raw data
}

// PluginError represents an error from plugin execution
// Maps to the WIT error record
type PluginError struct {
	Code    string
	Message string
}

// Error implements the error interface
func (e *PluginError) Error() string {
	return e.Code + ": " + e.Message
}

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
	FieldType   string // JSON Schema type: string, integer, boolean, object, array
	Required    bool
	Description string
}

// ObservationResult is the result of running an observation
type ObservationResult struct {
	Evidence *Evidence
	Error    *PluginError
}
