package engine

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/expr-lang/expr/vm"
	"github.com/whiskeyjimbo/reglet/internal/config"
	"github.com/whiskeyjimbo/reglet/internal/domain"
	"github.com/whiskeyjimbo/reglet/internal/domain/execution"
	"github.com/whiskeyjimbo/reglet/internal/domain/repositories"
	"github.com/whiskeyjimbo/reglet/internal/domain/services"
	"github.com/whiskeyjimbo/reglet/internal/redaction"
	"github.com/whiskeyjimbo/reglet/internal/wasm"
	"github.com/whiskeyjimbo/reglet/internal/wasm/hostfuncs"
	"golang.org/x/sync/errgroup"
)

// ExecutionConfig controls execution behavior.
type ExecutionConfig struct {
	// MaxConcurrentControls limits parallel control execution (0 = no limit)
	MaxConcurrentControls int
	// MaxConcurrentObservations limits parallel observation execution within a control (0 = no limit)
	MaxConcurrentObservations int
	// Parallel enables parallel execution (default: true for performance)
	Parallel bool

	// Include Filters (OR logic within slice, AND between types)
	IncludeTags       []string
	IncludeSeverities []string
	IncludeControlIDs []string // Exclusive - if set, other filters ignored

	// Exclude Filters (take precedence over includes)
	ExcludeTags       []string
	ExcludeControlIDs []string

	// Advanced Filter (Compiled Expression)
	FilterProgram *vm.Program

	// Dependency Strategy
	IncludeDependencies bool
}

// DefaultExecutionConfig returns sensible defaults for parallel execution.
func DefaultExecutionConfig() ExecutionConfig {
	return ExecutionConfig{
		MaxConcurrentControls:     10, // Reasonable default
		MaxConcurrentObservations: 5,  // Conservative to avoid overwhelming systems
		Parallel:                  true,
	}
}

// Engine coordinates profile execution.
type Engine struct {
	runtime    *wasm.Runtime
	executor   *ObservationExecutor
	config     ExecutionConfig
	repository repositories.ExecutionResultRepository // Optional repository for persistence
}

// CapabilityManager defines the interface for capability management
type CapabilityManager interface {
	CollectRequiredCapabilities(ctx context.Context, profile *config.Profile, runtime *wasm.Runtime, pluginDir string) (map[string][]hostfuncs.Capability, error)
	GrantCapabilities(required map[string][]hostfuncs.Capability) (map[string][]hostfuncs.Capability, error)
}

// NewEngine creates a new execution engine with default configuration.
func NewEngine(ctx context.Context) (*Engine, error) {
	return NewEngineWithConfig(ctx, DefaultExecutionConfig())
}

// NewEngineWithCapabilities creates an engine with interactive capability prompts
// and optional repository support.
func NewEngineWithCapabilities(
	ctx context.Context,
	capMgr CapabilityManager,
	pluginDir string,
	profile *config.Profile,
	cfg ExecutionConfig,
	redactor *redaction.Redactor,
	repo repositories.ExecutionResultRepository, // Optional repository
) (*Engine, error) {
	// Create temporary runtime with no capabilities to load plugins and get requirements
	tempRuntime, err := wasm.NewRuntime(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary runtime: %w", err)
	}

	// Collect required capabilities from all plugins
	required, err := capMgr.CollectRequiredCapabilities(ctx, profile, tempRuntime, pluginDir)
	if err != nil {
		_ = tempRuntime.Close(ctx) // Best-effort cleanup
		return nil, fmt.Errorf("failed to collect capabilities: %w", err)
	}

	// Close temporary runtime
	_ = tempRuntime.Close(ctx) // Best-effort cleanup

	// Get granted capabilities (will prompt user if needed)
	granted, err := capMgr.GrantCapabilities(required)
	if err != nil {
		return nil, fmt.Errorf("failed to grant capabilities: %w", err)
	}

	// Create WASM runtime with granted capabilities
	runtime, err := wasm.NewRuntimeWithCapabilities(ctx, granted)
	if err != nil {
		return nil, fmt.Errorf("failed to create WASM runtime: %w", err)
	}

	// Create observation executor
	executor := NewExecutor(runtime, pluginDir, redactor)

	// Preload plugins for schema validation
	for _, ctrl := range profile.Controls.Items {
		for _, obs := range ctrl.Observations {
			if _, err := executor.LoadPlugin(ctx, obs.Plugin); err != nil {
				return nil, fmt.Errorf("failed to preload plugin %s: %w", obs.Plugin, err)
			}
		}
	}

	return &Engine{
		runtime:    runtime,
		executor:   executor,
		config:     cfg,
		repository: repo,
	}, nil
}

// NewEngineWithConfig creates a new execution engine with custom configuration.
func NewEngineWithConfig(ctx context.Context, cfg ExecutionConfig) (*Engine, error) {
	// Create WASM runtime with no capabilities (legacy path)
	runtime, err := wasm.NewRuntime(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create WASM runtime: %w", err)
	}

	// Create observation executor with no redactor
	executor := NewObservationExecutor(runtime, nil)

	return &Engine{
		runtime:  runtime,
		executor: executor,
		config:   cfg,
	}, nil
}

// Execute runs a complete profile and returns the result.
func (e *Engine) Execute(ctx context.Context, profile *config.Profile) (*execution.ExecutionResult, error) {
	// Create execution result
	result := execution.NewExecutionResult(profile.Metadata.Name, profile.Metadata.Version)

	// Calculate required dependencies if enabled
	var requiredControls map[string]bool
	if e.config.IncludeDependencies {
		var err error
		requiredControls, err = e.resolveDependencies(profile)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve dependencies: %w", err)
		}
	}

	// Execute controls
	if e.config.Parallel && len(profile.Controls.Items) > 1 {
		// Parallel execution of controls
		if err := e.executeControlsParallel(ctx, profile.Controls.Items, result, requiredControls); err != nil {
			return nil, err
		}
	} else {
		// Sequential execution of controls
		for _, ctrl := range profile.Controls.Items {
			controlResult := e.executeControl(ctx, ctrl, result, requiredControls)
			result.AddControlResult(controlResult)
		}
	}

	// Finalize result (calculate summary, set end time)
	result.Finalize()

	// Persist result if repository is configured
	if e.repository != nil {
		if err := e.repository.Save(ctx, result); err != nil {
			// Log error but don't fail execution
			slog.Warn("failed to persist execution result", "error", err, "execution_id", result.GetID())
		}
	}

	return result, nil
}

// resolveDependencies calculates the transitive closure of dependencies for matched controls.
func (e *Engine) resolveDependencies(profile *config.Profile) (map[string]bool, error) {
	resolver := services.NewDependencyResolver()
	allDependencies, err := resolver.ResolveDependencies(profile.Controls.Items)
	if err != nil {
		return nil, err
	}

	required := make(map[string]bool)

	// Identify initial targets (controls that match filters)
	for _, ctrl := range profile.Controls.Items {
		if should, _ := e.shouldRun(ctrl); should {
			// Add all transitive dependencies for this control to required set
			if deps, ok := allDependencies[ctrl.ID]; ok {
				for depID := range deps {
					required[depID] = true
				}
			}
		}
	}

	return required, nil
}

// executeControlsParallel executes controls in parallel, respecting dependencies.
// Controls are organized into levels by BuildControlDAG, and each level is executed
// sequentially while controls within a level run in parallel.
func (e *Engine) executeControlsParallel(ctx context.Context, controls []config.Control, result *execution.ExecutionResult, requiredDeps map[string]bool) error {
	// Build dependency graph and get control levels
	resolver := services.NewDependencyResolver()
	levels, err := resolver.BuildControlDAG(controls)
	if err != nil {
		return fmt.Errorf("failed to build control dependency graph: %w", err)
	}

	// Execute each level sequentially, controls within level in parallel
	for _, level := range levels {
		// Create errgroup for this level
		g, levelCtx := errgroup.WithContext(ctx)

		// Apply concurrency limit if specified
		if e.config.MaxConcurrentControls > 0 {
			g.SetLimit(e.config.MaxConcurrentControls)
		}

		// Execute all controls in this level in parallel
		for _, ctrl := range level.Controls {
			g.Go(func() error {
				controlResult := e.executeControl(levelCtx, ctrl, result, requiredDeps)
				result.AddControlResult(controlResult)
				return nil // Don't fail fast on individual control errors
			})
		}

		// Wait for all controls in this level to complete before moving to next level
		if err := g.Wait(); err != nil {
			return fmt.Errorf("level %d execution failed: %w", level.Level, err)
		}
	}

	return nil
}

// executeControl executes a single control and returns its result.
func (e *Engine) executeControl(ctx context.Context, ctrl config.Control, execResult *execution.ExecutionResult, requiredDeps map[string]bool) execution.ControlResult {
	startTime := time.Now()

	result := execution.ControlResult{
		ID:           ctrl.ID,
		Name:         ctrl.Name,
		Description:  ctrl.Description,
		Severity:     ctrl.Severity,
		Tags:         ctrl.Tags,
		Observations: make([]execution.ObservationResult, 0, len(ctrl.Observations)),
	}

	// Check if control should run (filtering)
	shouldRun, skipReason := e.shouldRun(ctrl)

	// If filtering says skip, check if it's required as a dependency
	if !shouldRun && e.config.IncludeDependencies && requiredDeps[ctrl.ID] {
		shouldRun = true
		skipReason = "" // Clear skip reason as we are running it
	}

	if !shouldRun {
		result.Status = domain.StatusSkipped
		result.SkipReason = skipReason
		result.Message = skipReason
		result.Duration = time.Since(startTime)
		return result
	}

	// Check dependencies before execution
	if len(ctrl.DependsOn) > 0 {
		// Look up dependency statuses from previous results
		for _, depID := range ctrl.DependsOn {
			depStatus, found := execResult.GetControlStatus(depID)

			// If dependency not found or failed/error, skip this control
			if !found || depStatus == domain.StatusFail || depStatus == domain.StatusError || depStatus == domain.StatusSkipped {
				result.Status = domain.StatusSkipped
				if !found {
					result.Message = fmt.Sprintf("Skipped: dependency '%s' not found", depID)
				} else {
					result.Message = fmt.Sprintf("Skipped: dependency '%s' has status '%s'", depID, depStatus)
				}
				result.Duration = time.Since(startTime)
				return result
			}
		}
	}

	// Execute observations
	if e.config.Parallel && len(ctrl.Observations) > 1 {
		// Parallel execution of observations
		result.Observations = e.executeObservationsParallel(ctx, ctrl.Observations)
	} else {
		// Sequential execution of observations
		for _, obs := range ctrl.Observations {
			obsResult := e.executor.Execute(ctx, obs)
			result.Observations = append(result.Observations, obsResult)
		}
	}

	// Aggregate observation results to determine control status
	// Extract statuses from observations
	observationStatuses := make([]domain.Status, len(result.Observations))
	for i, obs := range result.Observations {
		observationStatuses[i] = obs.Status
	}

	aggregator := services.NewStatusAggregator()
	result.Status = aggregator.AggregateControlStatus(observationStatuses)

	// Generate message based on status
	result.Message = generateControlMessage(result.Status, result.Observations)

	// Set duration
	result.Duration = time.Since(startTime)

	return result
}

// shouldRun determines if a control should run based on the configuration filters.
func (e *Engine) shouldRun(ctrl config.Control) (bool, string) {
	filter := services.NewControlFilter().
		WithExclusiveControls(e.config.IncludeControlIDs).
		WithExcludedControls(e.config.ExcludeControlIDs).
		WithExcludedTags(e.config.ExcludeTags).
		WithIncludedTags(e.config.IncludeTags).
		WithIncludedSeverities(e.config.IncludeSeverities).
		WithFilterExpression(e.config.FilterProgram)

	return filter.ShouldRun(ctrl)
}

// executeObservationsParallel executes observations in parallel with concurrency limits.
func (e *Engine) executeObservationsParallel(ctx context.Context, observations []config.Observation) []execution.ObservationResult {
	g, ctx := errgroup.WithContext(ctx)

	// Apply concurrency limit if specified
	if e.config.MaxConcurrentObservations > 0 {
		g.SetLimit(e.config.MaxConcurrentObservations)
	}

	// Pre-allocate results slice to exact size needed
	// Each goroutine writes to a unique index - no mutex needed
	results := make([]execution.ObservationResult, len(observations))

	// Execute each observation in parallel
	for i, obs := range observations {
		g.Go(func() error {
			obsResult := e.executor.Execute(ctx, obs)
			// Safe without mutex: each goroutine writes to unique index in pre-allocated slice
			results[i] = obsResult
			return nil // Don't fail fast on individual observation errors
		})
	}

	// Wait for all observations to complete
	_ = g.Wait() // Ignore error as we don't fail fast

	return results
}

// generateControlMessage generates a human-readable message for the control result.
func generateControlMessage(status domain.Status, observations []execution.ObservationResult) string {
	switch status {
	case domain.StatusPass:
		if len(observations) == 1 {
			return "Check passed"
		}
		return fmt.Sprintf("All %d checks passed", len(observations))

	case domain.StatusFail:
		failCount := 0
		for _, obs := range observations {
			if obs.Status == domain.StatusFail {
				failCount++
			}
		}
		if failCount == 1 {
			return "1 check failed"
		}
		return fmt.Sprintf("%d checks failed", failCount)

	case domain.StatusError:
		errorCount := 0
		for _, obs := range observations {
			if obs.Status == domain.StatusError {
				errorCount++
			}
		}
		if errorCount == 1 {
			// Return the specific error message
			for _, obs := range observations {
				if obs.Status == domain.StatusError && obs.Error != nil {
					return obs.Error.Message
				}
			}
			return "Check encountered an error"
		}
		return fmt.Sprintf("%d checks encountered errors", errorCount)

	case domain.StatusSkipped:
		return "Skipped due to failed dependency"

	default:
		return "Unknown status"
	}
}

// Runtime returns the WASM runtime for accessing plugin schemas.
// This is used for pre-flight schema validation.
func (e *Engine) Runtime() *wasm.Runtime {
	return e.runtime
}

// Close closes the engine and releases resources.
func (e *Engine) Close(ctx context.Context) error {
	return e.runtime.Close(ctx)
}