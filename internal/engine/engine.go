package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jrose/reglet/internal/config"
	"github.com/jrose/reglet/internal/wasm"
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
	runtime  *wasm.Runtime
	executor *ObservationExecutor
	config   ExecutionConfig
}

// NewEngine creates a new execution engine with default configuration.
func NewEngine(ctx context.Context) (*Engine, error) {
	return NewEngineWithConfig(ctx, DefaultExecutionConfig())
}

// NewEngineWithConfig creates a new execution engine with custom configuration.
func NewEngineWithConfig(ctx context.Context, cfg ExecutionConfig) (*Engine, error) {
	// Create WASM runtime
	runtime, err := wasm.NewRuntime(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create WASM runtime: %w", err)
	}

	// Create observation executor
	executor := NewObservationExecutor(runtime)

	return &Engine{
		runtime:  runtime,
		executor: executor,
		config:   cfg,
	}, nil
}

// Execute runs a complete profile and returns the result.
func (e *Engine) Execute(ctx context.Context, profile *config.Profile) (*ExecutionResult, error) {
	// Create execution result
	result := NewExecutionResult(profile.Metadata.Name, profile.Metadata.Version)

	// Execute controls
	if e.config.Parallel && len(profile.Controls.Items) > 1 {
		// Parallel execution of controls
		if err := e.executeControlsParallel(ctx, profile.Controls.Items, result); err != nil {
			return nil, err
		}
	} else {
		// Sequential execution of controls
		for _, ctrl := range profile.Controls.Items {
			controlResult := e.executeControl(ctx, ctrl, result)
			result.AddControlResult(controlResult)
		}
	}

	// Finalize result (calculate summary, set end time)
	result.Finalize()

	return result, nil
}

// executeControlsParallel executes controls in parallel, respecting dependencies.
// Controls are organized into levels by BuildControlDAG, and each level is executed
// sequentially while controls within a level run in parallel.
func (e *Engine) executeControlsParallel(ctx context.Context, controls []config.Control, result *ExecutionResult) error {
	// Build dependency graph and get control levels
	levels, err := BuildControlDAG(controls)
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
				controlResult := e.executeControl(levelCtx, ctrl, result)
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
func (e *Engine) executeControl(ctx context.Context, ctrl config.Control, execResult *ExecutionResult) ControlResult {
	startTime := time.Now()

	result := ControlResult{
		ID:           ctrl.ID,
		Name:         ctrl.Name,
		Description:  ctrl.Description,
		Severity:     ctrl.Severity,
		Tags:         ctrl.Tags,
		Observations: make([]ObservationResult, 0, len(ctrl.Observations)),
	}

	// Check dependencies before execution
	if len(ctrl.DependsOn) > 0 {
		// Look up dependency statuses from previous results
		for _, depID := range ctrl.DependsOn {
			depStatus, found := execResult.GetControlStatus(depID)

			// If dependency not found or failed/error, skip this control
			if !found || depStatus == StatusFail || depStatus == StatusError || depStatus == StatusSkipped {
				result.Status = StatusSkipped
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
	result.Status = aggregateControlStatus(result.Observations)

	// Generate message based on status
	result.Message = generateControlMessage(result.Status, result.Observations)

	// Set duration
	result.Duration = time.Since(startTime)

	return result
}

// executeObservationsParallel executes observations in parallel with concurrency limits.
func (e *Engine) executeObservationsParallel(ctx context.Context, observations []config.Observation) []ObservationResult {
	g, ctx := errgroup.WithContext(ctx)

	// Apply concurrency limit if specified
	if e.config.MaxConcurrentObservations > 0 {
		g.SetLimit(e.config.MaxConcurrentObservations)
	}

	// Create results slice with mutex for thread-safe append
	results := make([]ObservationResult, len(observations))
	var mu sync.Mutex

	// Execute each observation in parallel
	for i, obs := range observations {
		g.Go(func() error {
			obsResult := e.executor.Execute(ctx, obs)
			mu.Lock()
			results[i] = obsResult
			mu.Unlock()
			return nil // Don't fail fast on individual observation errors
		})
	}

	// Wait for all observations to complete
	_ = g.Wait() // Ignore error as we don't fail fast

	return results
}

// aggregateControlStatus determines overall control status from observations.
// Logic for Phase 1b:
//   - If any observation is StatusError → Control is StatusError
//   - If all observations are StatusPass → Control is StatusPass
//   - If any observation is StatusFail → Control is StatusFail
func aggregateControlStatus(observations []ObservationResult) Status {
	if len(observations) == 0 {
		return StatusError
	}

	hasError := false
	hasFail := false
	passCount := 0

	for _, obs := range observations {
		switch obs.Status {
		case StatusError:
			hasError = true
		case StatusFail:
			hasFail = true
		case StatusPass:
			passCount++
		}
	}

	// Errors take precedence
	if hasError {
		return StatusError
	}

	// If any failures, control fails
	if hasFail {
		return StatusFail
	}

	// All observations passed
	if passCount == len(observations) {
		return StatusPass
	}

	// Fallback (shouldn't reach here)
	return StatusError
}

// generateControlMessage generates a human-readable message for the control result.
func generateControlMessage(status Status, observations []ObservationResult) string {
	switch status {
	case StatusPass:
		if len(observations) == 1 {
			return "Check passed"
		}
		return fmt.Sprintf("All %d checks passed", len(observations))

	case StatusFail:
		failCount := 0
		for _, obs := range observations {
			if obs.Status == StatusFail {
				failCount++
			}
		}
		if failCount == 1 {
			return "1 check failed"
		}
		return fmt.Sprintf("%d checks failed", failCount)

	case StatusError:
		errorCount := 0
		for _, obs := range observations {
			if obs.Status == StatusError {
				errorCount++
			}
		}
		if errorCount == 1 {
			// Return the specific error message
			for _, obs := range observations {
				if obs.Status == StatusError && obs.Error != nil {
					return obs.Error.Message
				}
			}
			return "Check encountered an error"
		}
		return fmt.Sprintf("%d checks encountered errors", errorCount)

	case StatusSkipped:
		return "Skipped due to failed dependency"

	default:
		return "Unknown status"
	}
}

// Close closes the engine and releases resources.
func (e *Engine) Close(ctx context.Context) error {
	return e.runtime.Close(ctx)
}
