package engine

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
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

	// Now that wasmResult is *ObservationResult from internal/wasm/plugin.go
	// It already contains wasm.Evidence (with Status, Error, Data) and potentially wasm.PluginError

	// If the plugin returned an error (Go error in observeFn.Call or processing failure)
	if wasmResult.Error != nil { // This error is a Go error, not from Evidence
		result.Status = StatusError
		result.Error = wasmResult.Error // Use the top-level error from wasmResult
		result.Duration = time.Since(startTime)
		return result
	}

	// Plugin returned evidence
	if wasmResult.Evidence != nil {
		result.Evidence = wasmResult.Evidence // Set the full Evidence from wasmResult

		// Determine status based on top-level Evidence.Status and expect expressions
		result.Status = determineStatusWithExpect(wasmResult.Evidence, obs.Expect)

		// Redact sensitive data from evidence before returning/storing it
		if e.redactor != nil && wasmResult.Evidence.Data != nil {
			redactedData := e.redactor.Redact(wasmResult.Evidence.Data)
			if asMap, ok := redactedData.(map[string]interface{}); ok {
				wasmResult.Evidence.Data = asMap
			}
		}

		// If the Evidence itself contains an error, propagate it to ObservationResult.Error
		if wasmResult.Evidence.Error != nil {
			result.Error = wasmResult.Evidence.Error
		}

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
// If no expect expressions provided, falls back to checking evidence.Status.
func determineStatusWithExpect(evidence *wasm.Evidence, expectations []string) Status {
	// If the evidence itself indicates an error, then the observation errored.
	if evidence.Error != nil {
		return StatusError
	}

	// If no expectations provided, fall back to checking top-level Evidence.Status
	if len(expectations) == 0 {
		return determineStatusFromEvidenceStatus(evidence)
	}

	// Create environment for expression evaluation
	// The evidence data is available under "data" namespace, plus top-level fields
	env := map[string]interface{}{
		"data":      evidence.Data,      // The original data map
		"status":    evidence.Status,    // Top-level status
		"timestamp": evidence.Timestamp, // Top-level timestamp
		"error":     evidence.Error,     // Top-level error
	}

	// Get common expr options including custom functions
	options := getExprOptions(env)

	// Evaluate all expect expressions
	// ALL expressions must evaluate to true for the observation to pass
	for _, expectExpr := range expectations {
		program, err := expr.Compile(expectExpr, options...)
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

	// All expressions evaluated to true and no top-level Evidence.Error
	return StatusPass
}

// getExprOptions returns a list of expr options including custom helper functions.
func getExprOptions(env map[string]interface{}) []expr.Option {
	return []expr.Option{
		expr.Env(env),
		expr.AsBool(),
		expr.Function("strContains", func(params ...interface{}) (interface{}, error) {
			if len(params) != 2 {
				return nil, fmt.Errorf("strContains expects 2 arguments")
			}
			s, ok := params[0].(string)
			if !ok {
				return nil, fmt.Errorf("strContains: first argument must be a string")
			}
			substr, ok := params[1].(string)
			if !ok {
				return nil, fmt.Errorf("strContains: second argument must be a string")
			}
			return strings.Contains(s, substr), nil
		}),
		expr.Function("isIPv4", func(params ...interface{}) (interface{}, error) {
			if len(params) != 1 {
				return nil, fmt.Errorf("isIPv4 expects 1 argument")
			}
			ipStr, ok := params[0].(string)
			if !ok {
				return nil, fmt.Errorf("isIPv4: argument must be a string")
			}
			ip := net.ParseIP(ipStr)
			return ip != nil && ip.To4() != nil, nil
		}),
	}
}

// determineStatusFromEvidenceStatus checks the top-level Evidence.Status field.
// Used as fallback when no expect expressions are provided.
func determineStatusFromEvidenceStatus(evidence *wasm.Evidence) Status {
	// If Evidence.Error is present, it implies an error status.
	if evidence.Error != nil {
		return StatusError
	}
	if evidence.Status {
		return StatusPass
	}
	return StatusFail
}
