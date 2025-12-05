package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/whiskeyjimbo/reglet/internal/config"
	"github.com/whiskeyjimbo/reglet/internal/wasm"
)

// ObservationExecutor executes observations using WASM plugins.
type ObservationExecutor struct {
	runtime   *wasm.Runtime
	pluginDir string // Base directory where plugins are located (e.g., /path/to/project/plugins)
}

// NewObservationExecutor creates a new observation executor with auto-detected plugin directory.
func NewObservationExecutor(runtime *wasm.Runtime) *ObservationExecutor {
	var pluginDirPath string

	// 1. Check Env Var (Best for production binaries)
	if envPath := os.Getenv("REGLET_PLUGIN_DIR"); envPath != "" {
		pluginDirPath = envPath
	} else {
		// 2. Fallback to dev mode logic
		projectRoot := findProjectRoot()
		pluginDirPath = filepath.Join(projectRoot, "plugins")
	}

	return &ObservationExecutor{
		runtime:   runtime,
		pluginDir: pluginDirPath,
	}
}

// NewExecutor creates a new observation executor with explicit plugin directory.
func NewExecutor(runtime *wasm.Runtime, pluginDir string) *ObservationExecutor {
	return &ObservationExecutor{
		runtime:   runtime,
		pluginDir: pluginDir,
	}
}

// findProjectRoot attempts to find the project root by looking for the go.mod file.
// It searches up to 5 parent directories from the current working directory.
func findProjectRoot() string {
	workingDir, err := os.Getwd()
	if err != nil {
		// Fallback to current directory if we can't get WD
		return "."
	}

	currentDir := workingDir
	for i := 0; i < 5; i++ { // Limit search to prevent infinite loops
		if _, err := os.Stat(filepath.Join(currentDir, "go.mod")); err == nil {
			return currentDir
		}
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir { // Reached file system root
			break
		}
		currentDir = parentDir
	}

	// If go.mod not found, assume current working directory is the base
	return workingDir
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
	// Pass config values directly without type conversion to preserve types (int, bool, etc.)
	wasmConfig := wasm.Config{
		Values: obs.Config,
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

	// Construct plugin path using the pre-calculated pluginDir
	// Plugin name is validated in config.validatePluginName() to prevent path traversal
	pluginPath := filepath.Join(e.pluginDir, pluginName, pluginName+".wasm")

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
