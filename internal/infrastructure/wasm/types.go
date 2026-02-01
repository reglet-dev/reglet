// Package wasm provides WebAssembly runtime infrastructure for Reglet plugins.
// It manages plugin loading, execution, and capability-based sandboxing using wazero.
package wasm

import (
	"github.com/reglet-dev/reglet/internal/domain/execution"
)

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

// PluginObservationResult is the result of running an observation through a WASM plugin.
// This is a low-level boundary type.
type PluginObservationResult struct {
	Evidence *execution.Evidence
	Error    *execution.PluginError
}
