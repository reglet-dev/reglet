// Package engine coordinates profile execution and validation.
package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/reglet-dev/reglet/internal/domain/capability"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/execution"
	"github.com/reglet-dev/reglet/internal/domain/repositories"
	"github.com/reglet-dev/reglet/internal/domain/services"
	"github.com/reglet-dev/reglet/internal/infrastructure/build"
	"github.com/reglet-dev/reglet/internal/infrastructure/sensitivedata"
	"github.com/reglet-dev/reglet/internal/infrastructure/wasm"
)

// ObservationExecutable defines the interface for executing observations.
type ObservationExecutable interface {
	Execute(ctx context.Context, obs entities.ObservationDefinition) execution.ObservationResult
}

// EngineOption configures an Engine during construction.
type EngineOption func(*engineOptions)

// engineOptions holds all configurable engine parameters.
type engineOptions struct {
	capabilityManager CapabilityManager
	capabilities      map[string]capability.GrantSet
	profile           entities.ProfileReader
	repository        repositories.ExecutionResultRepository
	truncator         execution.TruncationStrategy
	redactor          *sensitivedata.Redactor
	pluginDir         string
	executionConfig   ExecutionConfig
	memoryLimitMB     int
}

// defaultEngineOptions returns options with sensible defaults.
func defaultEngineOptions() *engineOptions {
	return &engineOptions{
		executionConfig: DefaultExecutionConfig(),
		truncator:       &execution.GreedyTruncator{},
		memoryLimitMB:   0,  // 0 = unlimited
		pluginDir:       "", // empty = auto-detect
	}
}

// WithExecutionConfig sets custom execution configuration.
func WithExecutionConfig(cfg ExecutionConfig) EngineOption {
	return func(o *engineOptions) {
		o.executionConfig = cfg
	}
}

// WithCapabilities sets the pre-granted capabilities for the engine.
func WithCapabilities(caps map[string]capability.GrantSet) EngineOption {
	return func(o *engineOptions) {
		o.capabilities = caps
	}
}

// WithCapabilityManager enables interactive capability prompts.
// Requires WithProfile to also be set.
func WithCapabilityManager(mgr CapabilityManager) EngineOption {
	return func(o *engineOptions) {
		o.capabilityManager = mgr
	}
}

// WithPluginDir sets the directory to search for external plugins.
// If empty, auto-detection is used.
func WithPluginDir(dir string) EngineOption {
	return func(o *engineOptions) {
		o.pluginDir = dir
	}
}

// WithProfile sets the profile for capability collection.
// Required when using WithCapabilityManager.
func WithProfile(p entities.ProfileReader) EngineOption {
	return func(o *engineOptions) {
		o.profile = p
	}
}

// WithRedactor enables sensitive data redaction in evidence.
func WithRedactor(r *sensitivedata.Redactor) EngineOption {
	return func(o *engineOptions) {
		o.redactor = r
	}
}

// WithRepository enables execution result persistence.
func WithRepository(repo repositories.ExecutionResultRepository) EngineOption {
	return func(o *engineOptions) {
		o.repository = repo
	}
}

// WithMemoryLimit sets the WASM memory limit in megabytes.
// Use 0 for unlimited (default).
func WithMemoryLimit(mb int) EngineOption {
	return func(o *engineOptions) {
		o.memoryLimitMB = mb
	}
}

// WithTruncator sets the evidence truncation strategy.
func WithTruncator(t execution.TruncationStrategy) EngineOption {
	return func(o *engineOptions) {
		o.truncator = t
	}
}

// Engine coordinates profile execution.
type Engine struct {
	repository  repositories.ExecutionResultRepository
	executor    ObservationExecutable
	truncator   execution.TruncationStrategy
	runtime     *wasm.Runtime
	profileVars map[string]interface{}
	version     build.Info
	config      ExecutionConfig
}

// CapabilityCollector collects required capabilities from plugins.
type CapabilityCollector interface {
	CollectRequiredCapabilities(ctx context.Context, profile entities.ProfileReader, runtime *wasm.Runtime, pluginDir string) (map[string]capability.GrantSet, error)
}

// CapabilityGranter grants capabilities (interactively or automatically).
type CapabilityGranter interface {
	GrantCapabilities(required map[string]capability.GrantSet) (map[string]capability.GrantSet, error)
}

// CapabilityManager combines collection and granting for convenience.
type CapabilityManager interface {
	CapabilityCollector
	CapabilityGranter
}

// NewEngine creates a new execution engine with optional configuration.
//
// Required parameters:
//   - ctx: execution context
//   - version: build version information
//
// Optional configuration via EngineOption functions:
//   - WithExecutionConfig: custom execution settings
//   - WithCapabilities: set pre-granted capabilities
//   - WithCapabilityManager: enable interactive capability prompts (requires WithProfile)
//   - WithPluginDir: custom plugin directory (defaults to auto-detect)
//   - WithProfile: profile for capability collection
//   - WithRedactor: enable sensitive data redaction
//   - WithRepository: enable execution result persistence
//   - WithMemoryLimit: WASM memory limit in MB (0 = unlimited)
//   - WithTruncator: evidence truncation strategy
//
// Examples:
//
//	# Simple engine with defaults
//	engine, err := NewEngine(ctx, version)
//
//	# Engine with custom config
//	engine, err := NewEngine(ctx, version,
//	    WithExecutionConfig(cfg),
//	    WithMemoryLimit(512),
//	)
//
//	# Full-featured engine with capabilities
//	engine, err := NewEngine(ctx, version,
//	    WithCapabilityManager(capMgr),
//	    WithProfile(profile),
//	    WithPluginDir("/custom/plugins"),
//	    WithRedactor(redactor),
//	    WithRepository(repo),
//	)
func NewEngine(ctx context.Context, version build.Info, opts ...EngineOption) (*Engine, error) {
	cfg := defaultEngineOptions()
	for _, opt := range opts {
		opt(cfg)
	}

	// Validate required combinations
	if cfg.capabilityManager != nil && cfg.profile == nil {
		return nil, fmt.Errorf("WithCapabilityManager requires WithProfile to be set")
	}

	// If capability manager is provided, use the capability flow
	if cfg.capabilityManager != nil && cfg.profile != nil {
		return newEngineWithCapabilities(ctx, version, cfg)
	}

	// Otherwise, create simple engine (possibly with pre-granted capabilities)
	return newEngineSimple(ctx, version, cfg)
}

// newEngineWithCapabilities creates an engine with capability prompting.
// This is the internal implementation of the capability flow.
func newEngineWithCapabilities(
	ctx context.Context,
	version build.Info,
	cfg *engineOptions,
) (*Engine, error) {
	// Create temporary runtime with no capabilities to load plugins and get requirements
	tempRuntime, err := wasm.NewRuntime(ctx, version)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary runtime: %w", err)
	}

	// Collect required capabilities from all plugins
	required, err := cfg.capabilityManager.CollectRequiredCapabilities(ctx, cfg.profile, tempRuntime, cfg.pluginDir)
	if err != nil {
		_ = tempRuntime.Close(ctx)
		return nil, fmt.Errorf("failed to collect capabilities: %w", err)
	}

	_ = tempRuntime.Close(ctx)

	// Get granted capabilities (will prompt user if needed)
	granted, err := cfg.capabilityManager.GrantCapabilities(required)
	if err != nil {
		return nil, fmt.Errorf("failed to grant capabilities: %w", err)
	}

	// Create WASM runtime with granted capabilities and redactor
	runtimeOpts := []wasm.RuntimeOption{
		wasm.WithCapabilities(granted),
	}
	if cfg.redactor != nil {
		runtimeOpts = append(runtimeOpts, wasm.WithRedactor(cfg.redactor))
	}
	if cfg.memoryLimitMB > 0 {
		runtimeOpts = append(runtimeOpts, wasm.WithMemoryLimit(cfg.memoryLimitMB))
	}

	runtime, err := wasm.NewRuntime(ctx, version, runtimeOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create WASM runtime: %w", err)
	}

	executorOpts := []ExecutorOption{}
	if cfg.pluginDir != "" {
		executorOpts = append(executorOpts, WithExecutorPluginDir(cfg.pluginDir))
	}
	if cfg.redactor != nil {
		executorOpts = append(executorOpts, WithExecutorRedactor(cfg.redactor))
	}

	executor := NewExecutor(runtime, executorOpts...)

	// Preload plugins for schema validation
	for _, ctrl := range cfg.profile.GetControls() {
		for _, obs := range ctrl.ObservationDefinitions {
			if _, err := executor.LoadPlugin(ctx, obs.Plugin); err != nil {
				return nil, fmt.Errorf("failed to preload plugin %s: %w", obs.Plugin, err)
			}
		}
	}

	return &Engine{
		runtime:     runtime,
		executor:    executor,
		config:      cfg.executionConfig,
		repository:  cfg.repository,
		version:     version,
		truncator:   cfg.truncator,
		profileVars: cfg.profile.GetVars(),
	}, nil
}

// newEngineSimple creates a basic engine without capability prompting.
// This is the internal implementation of the simple flow.
func newEngineSimple(
	ctx context.Context,
	version build.Info,
	cfg *engineOptions,
) (*Engine, error) {
	runtimeOpts := []wasm.RuntimeOption{}
	if cfg.capabilities != nil {
		runtimeOpts = append(runtimeOpts, wasm.WithCapabilities(cfg.capabilities))
	}
	if cfg.redactor != nil {
		runtimeOpts = append(runtimeOpts, wasm.WithRedactor(cfg.redactor))
	}
	if cfg.memoryLimitMB > 0 {
		runtimeOpts = append(runtimeOpts, wasm.WithMemoryLimit(cfg.memoryLimitMB))
	}

	runtime, err := wasm.NewRuntime(ctx, version, runtimeOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create WASM runtime: %w", err)
	}

	executorOpts := []ExecutorOption{}
	if cfg.pluginDir != "" {
		executorOpts = append(executorOpts, WithExecutorPluginDir(cfg.pluginDir))
	}
	if cfg.redactor != nil {
		executorOpts = append(executorOpts, WithExecutorRedactor(cfg.redactor))
	}

	executor := NewExecutor(runtime, executorOpts...)

	return &Engine{
		runtime:    runtime,
		executor:   executor,
		config:     cfg.executionConfig,
		repository: cfg.repository,
		version:    version,
		truncator:  cfg.truncator,
	}, nil
}

// checkContextCancellation checks if the context has been canceled or timed out.
// Returns an appropriate error if canceled, nil if still active.
func checkContextCancellation(ctx context.Context) error {
	if ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("execution timed out: %w", ctx.Err())
		}
		return ctx.Err()
	}
	return nil
}

// Execute runs a complete profile and returns the result.
func (e *Engine) Execute(ctx context.Context, profile entities.ProfileReader) (*execution.ExecutionResult, error) {
	// Check context before starting
	if err := checkContextCancellation(ctx); err != nil {
		return nil, err
	}

	// Store profile vars for loop expansion
	e.profileVars = profile.GetVars()

	metadata := profile.GetMetadata()
	result := execution.NewExecutionResult(metadata.Name, metadata.Version)
	result.RegletVersion = e.version.String()

	var requiredControls map[string]bool
	if e.config.IncludeDependencies {
		var err error
		requiredControls, err = e.resolveDependencies(profile)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve dependencies: %w", err)
		}
	}

	// Calculate runnable controls using static filters (ControlSet.Select)
	// This replaces the per-control static checks in shouldRun
	filteredControls := profile.GetControls().Select(
		entities.WithTags(e.config.IncludeTags...),
		entities.WithSeverities(e.config.IncludeSeverities...),
		entities.WithIDs(e.config.IncludeControlIDs...),
		entities.ExcludeTags(e.config.ExcludeTags...),
		entities.ExcludeIDs(e.config.ExcludeControlIDs...),
	)

	runnableIDs := make(map[string]bool)
	for _, ctrl := range filteredControls {
		runnableIDs[ctrl.ID] = true
	}

	allControls := profile.GetControls()
	if e.config.Parallel && len(allControls) > 1 {
		if err := e.executeControlsWithWorkerPool(ctx, allControls, result, requiredControls, runnableIDs); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, fmt.Errorf("execution timed out: %w", err)
			}
			return nil, err
		}
	} else {
		for i, ctrl := range allControls {
			// Check context in loop
			if err := checkContextCancellation(ctx); err != nil {
				return nil, err
			}

			controlResult := e.executeControl(ctx, ctrl, i, result, requiredControls, runnableIDs)
			result.AddControlResult(controlResult)
		}

		if err := checkContextCancellation(ctx); err != nil {
			return nil, err
		}
	}

	result.Finalize()

	if e.repository != nil {
		if err := e.repository.Save(ctx, result); err != nil {
			slog.Warn("failed to persist execution result (execution completed successfully, but audit trail may be incomplete)",
				"error", err,
				"execution_id", result.GetID(),
				"note", "results were not saved to repository - this does not affect execution correctness")
		}
	}

	return result, nil
}

// resolveDependencies calculates the transitive closure of dependencies for matched controls.
func (e *Engine) resolveDependencies(profile entities.ProfileReader) (map[string]bool, error) {
	resolver := services.NewDependencyResolver()
	allControls := profile.GetControls()
	allDependencies, err := resolver.ResolveDependencies(allControls)
	if err != nil {
		return nil, err
	}

	required := make(map[string]bool)

	for _, ctrl := range allControls {
		if should, _ := e.shouldRun(ctrl, nil); should {
			if deps, ok := allDependencies[ctrl.ID]; ok {
				for depID := range deps {
					required[depID] = true
				}
			}
		}
	}

	return required, nil
}

// Runtime returns the WASM runtime for accessing plugin schemas.
func (e *Engine) Runtime() *wasm.Runtime {
	return e.runtime
}

// Close closes the engine and releases resources.
func (e *Engine) Close(ctx context.Context) error {
	return e.runtime.Close(ctx)
}
