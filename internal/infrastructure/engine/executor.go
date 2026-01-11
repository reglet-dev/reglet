package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/execution"
	"github.com/reglet-dev/reglet/internal/domain/services"
	"github.com/reglet-dev/reglet/internal/domain/values"
	"github.com/reglet-dev/reglet/internal/infrastructure/sensitivedata"
	"github.com/reglet-dev/reglet/internal/infrastructure/wasm"
)

// ObservationExecutor executes observations using WASM plugins.
type ObservationExecutor struct {
	runtime        *wasm.Runtime
	redactor       *sensitivedata.Redactor
	pluginRegistry *entities.PluginRegistry
	pluginDir      string
}

// ExecutorOption configures an ObservationExecutor.
type ExecutorOption func(*ObservationExecutor)

// NewExecutor creates a new observation executor with optional configuration.
// By default, plugin directory is auto-detected and no redactor is used.
//
// Example usage:
//
//	// Simple case (auto-detect plugin dir, no redactor)
//	executor := NewExecutor(runtime)
//
//	// With explicit plugin dir
//	executor := NewExecutor(runtime, WithPluginDir(pluginDir))
//
//	// With everything
//	executor := NewExecutor(runtime,
//	    WithPluginDir(pluginDir),
//	    WithRedactor(redactor),
//	    WithPluginRegistry(registry),
//	)
func NewExecutor(runtime *wasm.Runtime, opts ...ExecutorOption) *ObservationExecutor {
	e := &ObservationExecutor{
		runtime: runtime,
	}

	// Apply options
	for _, opt := range opts {
		opt(e)
	}

	// Auto-detect plugin dir if not provided
	if e.pluginDir == "" {
		e.pluginDir = autoDetectPluginDir()
	}

	return e
}

// WithPluginDir sets the plugin directory explicitly.
func WithPluginDir(dir string) ExecutorOption {
	return func(e *ObservationExecutor) {
		e.pluginDir = dir
	}
}

// WithRedactor enables secret redaction.
func WithRedactor(redactor *sensitivedata.Redactor) ExecutorOption {
	return func(e *ObservationExecutor) {
		e.redactor = redactor
	}
}

// WithPluginRegistry enables plugin alias resolution.
func WithPluginRegistry(registry *entities.PluginRegistry) ExecutorOption {
	return func(e *ObservationExecutor) {
		e.pluginRegistry = registry
	}
}

// autoDetectPluginDir attempts to find the plugin directory.
func autoDetectPluginDir() string {
	// 1. Check Env Var (Best for production binaries)
	if envPath := os.Getenv("REGLET_PLUGIN_DIR"); envPath != "" {
		return envPath
	}
	// 2. Fallback to dev mode logic
	projectRoot := findProjectRoot()
	return filepath.Join(projectRoot, "plugins")
}

// SetPluginRegistry sets the plugin registry for alias resolution.
func (e *ObservationExecutor) SetPluginRegistry(registry *entities.PluginRegistry) {
	e.pluginRegistry = registry
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
func (e *ObservationExecutor) Execute(ctx context.Context, obs entities.ObservationDefinition) execution.ObservationResult {
	startTime := time.Now()

	result := execution.ObservationResult{
		Plugin:   obs.Plugin,
		Config:   obs.Config,
		Duration: 0,
	}

	// Load the plugin
	plugin, err := e.LoadPlugin(ctx, obs.Plugin)
	if err != nil {
		result.Status = values.StatusError
		result.Error = &wasm.PluginError{
			Code:    "plugin_load_error",
			Message: err.Error(),
		}
		result.RawError = err
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
		result.Status = values.StatusError
		result.Error = &wasm.PluginError{
			Code:    "plugin_execution_error",
			Message: err.Error(),
		}
		result.RawError = err
		result.Duration = time.Since(startTime)
		return result
	}

	// Now that wasmResult is *ObservationResult from internal/wasm/plugin.go
	// It already contains wasm.Evidence (with Status, Error, Data) and potentially wasm.PluginError

	// If the plugin returned an error (Go error in observeFn.Call or processing failure)
	if wasmResult.Error != nil { // This error is a Go error, not from Evidence
		result.Status = values.StatusError
		result.Error = wasmResult.Error // Use the top-level error from wasmResult
		result.RawError = wasmResult.Error
		result.Duration = time.Since(startTime)
		return result
	}

	// Plugin returned evidence
	if wasmResult.Evidence != nil {
		result.Evidence = wasmResult.Evidence // Set the full Evidence from wasmResult

		// Determine status based on top-level Evidence.Status and expect expressions
		status, expectations := e.determineStatusWithExpect(ctx, wasmResult, obs.Expect)
		result.Status = status
		result.Expectations = expectations

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
	result.Status = values.StatusError
	result.Error = &wasm.PluginError{
		Code:    "invalid_plugin_result",
		Message: "plugin returned neither evidence nor error",
	}
	result.Duration = time.Since(startTime)
	return result
}

// LoadPlugin loads a plugin by name or alias.
// If a plugin registry is set, aliases are resolved to their actual plugin names.
// Phase 1b loads from file system. Phase 2 will use embedded plugins.
func (e *ObservationExecutor) LoadPlugin(ctx context.Context, pluginName string) (*wasm.Plugin, error) {
	// Resolve alias if registry is set
	resolvedName := pluginName
	if e.pluginRegistry != nil {
		spec := e.pluginRegistry.Resolve(pluginName)
		resolvedName = spec.PluginName()
	}

	// Check if already loaded in runtime cache (check both alias and resolved name)
	if plugin, ok := e.runtime.GetPlugin(pluginName); ok {
		return plugin, nil
	}
	if resolvedName != pluginName {
		if plugin, ok := e.runtime.GetPlugin(resolvedName); ok {
			return plugin, nil
		}
	}

	// Validate plugin name to prevent path traversal
	// NewPluginName enforces strict character set (alphanumeric, _, -) and no paths
	validName, err := values.NewPluginName(resolvedName)
	if err != nil {
		return nil, fmt.Errorf("invalid plugin name %q (resolved from %q): %w", resolvedName, pluginName, err)
	}

	// Construct plugin path using the pre-calculated pluginDir
	// Use validated value to ensure safety
	safeName := validName.String()
	pluginPath := filepath.Join(e.pluginDir, safeName, safeName+".wasm")

	// Read the WASM file
	//nolint:gosec // G304: pluginPath is constructed from validated pluginName (alphanumeric only)
	wasmBytes, err := os.ReadFile(pluginPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin %s: %w (expected at %s)", resolvedName, err, pluginPath)
	}

	// Load the plugin into the runtime with the alias as the key for caching
	return e.runtime.LoadPlugin(ctx, pluginName, wasmBytes)
}

// determineStatusWithExpect determines the observation status by evaluating expect expressions.
func (e *ObservationExecutor) determineStatusWithExpect(ctx context.Context, wasmResult *wasm.PluginObservationResult, expects []string) (values.Status, []execution.ExpectationResult) {
	aggregator := services.NewStatusAggregator()
	return aggregator.DetermineObservationStatus(ctx, wasmResult.Evidence, expects)
}
