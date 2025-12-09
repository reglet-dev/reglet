package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/expr-lang/expr"
	"github.com/whiskeyjimbo/reglet/internal/config"
	"github.com/whiskeyjimbo/reglet/internal/redaction"
	"github.com/whiskeyjimbo/reglet/internal/wasm"
)

// ObservationExecutor executes observations using WASM plugins.
type ObservationExecutor struct {
	runtime   *wasm.Runtime
	pluginDir string // Base directory where plugins are located (e.g., /path/to/project/plugins)
	redactor  *redaction.Redactor
}

// NewObservationExecutor creates a new observation executor with auto-detected plugin directory.
func NewObservationExecutor(runtime *wasm.Runtime, redactor *redaction.Redactor) *ObservationExecutor {
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
		redactor:  redactor,
	}
}

// NewExecutor creates a new observation executor with explicit plugin directory.
func NewExecutor(runtime *wasm.Runtime, pluginDir string, redactor *redaction.Redactor) *ObservationExecutor {
	return &ObservationExecutor{
		runtime:   runtime,
		pluginDir: pluginDir,
		redactor:  redactor,
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
	plugin, err := e.LoadPlugin(ctx, obs.Plugin)
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
		// Determine status BEFORE redaction to ensure tests run against raw data
		// Evaluate expect expressions if provided, otherwise fall back to status field
		result.Status = determineStatusWithExpect(wasmResult.Evidence, obs.Expect)

		// Redact sensitive data from evidence before returning/storing it
		if e.redactor != nil && wasmResult.Evidence.Data != nil {
			redactedData := e.redactor.Redact(wasmResult.Evidence.Data)
			if asMap, ok := redactedData.(map[string]interface{}); ok {
				wasmResult.Evidence.Data = asMap
			}
		}

		result.Evidence = wasmResult.Evidence
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

// LoadPlugin loads a plugin by name.
// Phase 1b loads from file system. Phase 2 will use embedded plugins.
func (e *ObservationExecutor) LoadPlugin(ctx context.Context, pluginName string) (*wasm.Plugin, error) {
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

// determineStatusWithExpect determines the observation status by evaluating expect expressions.
// If no expect expressions provided, falls back to checking evidence.Data["status"].
func determineStatusWithExpect(evidence *wasm.Evidence, expectations []string) Status {
	if evidence.Data == nil {
		return StatusError
	}

	// If no expectations provided, fall back to checking status field
	if len(expectations) == 0 {
		return determineStatusFromField(evidence)
	}

	// Create environment for expression evaluation
	// The evidence data is available under "data" namespace
	env := map[string]interface{}{
		"data": evidence.Data,
	}

	// Evaluate all expect expressions
	// ALL expressions must evaluate to true for the observation to pass
	for _, expectExpr := range expectations {
		program, err := expr.Compile(expectExpr, expr.Env(env), expr.AsBool())
		if err != nil {
			// Expression compilation error - treat as error
			return StatusError
		}

		result, err := expr.Run(program, env)
		if err != nil {
			// Expression evaluation error - treat as error
			return StatusError
		}

		// Check if result is boolean true
		resultBool, ok := result.(bool)
		if !ok {
			// Expression didn't return boolean - treat as error
			return StatusError
		}

		// If any expression is false, observation fails
		if !resultBool {
			return StatusFail
		}
	}

	// All expressions evaluated to true
	return StatusPass
}

// determineStatusFromField checks the evidence.Data["status"] field.
// Used as fallback when no expect expressions are provided.
func determineStatusFromField(evidence *wasm.Evidence) Status {
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
