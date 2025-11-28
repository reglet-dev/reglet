package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jrose/reglet/internal/config"
	"github.com/jrose/reglet/internal/wasm"
)

// ObservationExecutor executes observations using WASM plugins.
type ObservationExecutor struct {
	runtime *wasm.Runtime
}

// NewObservationExecutor creates a new observation executor.
func NewObservationExecutor(runtime *wasm.Runtime) *ObservationExecutor {
	return &ObservationExecutor{
		runtime: runtime,
	}
}

// Execute runs a single observation and returns the result.
func (e *ObservationExecutor) Execute(ctx context.Context, obs config.Observation) ObservationResult {
	startTime := time.Now()

	result := ObservationResult{
		Plugin:   obs.Plugin,
		Config:   obs.Config,
		Duration: 0,
	}

	// Load the plugin
	plugin, err := e.loadPlugin(ctx, obs.Plugin)
	if err != nil {
		result.Status = StatusError
		result.Error = &wasm.PluginError{
			Code:    "plugin_load_error",
			Message: err.Error(),
		}
		result.Duration = time.Since(startTime)
		return result
	}

	// Convert observation config to WASM config
	wasmConfig := wasm.Config{
		Values: make(map[string]string),
	}

	// Convert config map to string values
	for key, value := range obs.Config {
		wasmConfig.Values[key] = fmt.Sprintf("%v", value)
	}

	// Execute the observation
	wasmResult, err := plugin.Observe(ctx, wasmConfig)
	if err != nil {
		result.Status = StatusError
		result.Error = &wasm.PluginError{
			Code:    "plugin_execution_error",
			Message: err.Error(),
		}
		result.Duration = time.Since(startTime)
		return result
	}

	// Check if plugin returned an error
	if wasmResult.Error != nil {
		result.Status = StatusError
		result.Error = wasmResult.Error
		result.Duration = time.Since(startTime)
		return result
	}

	// Plugin returned evidence
	if wasmResult.Evidence != nil {
		result.Evidence = wasmResult.Evidence
		result.Status = determineStatus(wasmResult.Evidence)
		result.Duration = time.Since(startTime)
		return result
	}

	// Neither error nor evidence (unexpected)
	result.Status = StatusError
	result.Error = &wasm.PluginError{
		Code:    "invalid_plugin_result",
		Message: "plugin returned neither evidence nor error",
	}
	result.Duration = time.Since(startTime)
	return result
}

// loadPlugin loads a plugin by name.
// Phase 1b loads from file system. Phase 2 will use embedded plugins.
func (e *ObservationExecutor) loadPlugin(ctx context.Context, pluginName string) (*wasm.Plugin, error) {
	// Check if already loaded in runtime cache
	if plugin, ok := e.runtime.GetPlugin(pluginName); ok {
		return plugin, nil
	}

	// Find the project root by looking for go.mod
	workingDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	// Dynamically construct plugin path: plugins/{pluginName}/{pluginName}.wasm
	// This works for both running from project root and from subdirectories
	pluginPath := filepath.Join(workingDir, "plugins", pluginName, pluginName+".wasm")

	// If not found, try going up directories to find project root
	for i := 0; i < 5; i++ {
		if _, err := os.Stat(pluginPath); err == nil {
			break
		}
		workingDir = filepath.Dir(workingDir)
		pluginPath = filepath.Join(workingDir, "plugins", pluginName, pluginName+".wasm")
	}

	// Read the WASM file
	wasmBytes, err := os.ReadFile(pluginPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin %s: %w (expected at %s)", pluginName, err, pluginPath)
	}

	// Load the plugin into the runtime
	return e.runtime.LoadPlugin(ctx, pluginName, wasmBytes)
}

// determineStatus determines the observation status from evidence.
// Phase 1b uses simple logic: check if evidence.Data["status"] is true/false.
// Phase 2 will use expect expression evaluation.
func determineStatus(evidence *wasm.Evidence) Status {
	if evidence.Data == nil {
		return StatusError
	}

	// Look for "status" field in evidence data
	statusValue, ok := evidence.Data["status"]
	if !ok {
		// No status field - treat as error
		return StatusError
	}

	// Check if status is boolean true/false
	statusBool, ok := statusValue.(bool)
	if !ok {
		// Status field is not a boolean - treat as error
		return StatusError
	}

	if statusBool {
		return StatusPass
	}
	return StatusFail
}
