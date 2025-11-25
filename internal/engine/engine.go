package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/jrose/reglet/internal/config"
	"github.com/jrose/reglet/internal/wasm"
)

// Engine coordinates profile execution.
type Engine struct {
	runtime  *wasm.Runtime
	executor *ObservationExecutor
}

// NewEngine creates a new execution engine.
func NewEngine(ctx context.Context) (*Engine, error) {
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
	}, nil
}

// Execute runs a complete profile and returns the result.
func (e *Engine) Execute(ctx context.Context, profile *config.Profile) (*ExecutionResult, error) {
	// Create execution result
	result := NewExecutionResult(profile.Metadata.Name, profile.Metadata.Version)

	// Execute each control sequentially (Phase 1b - no parallelism)
	for _, ctrl := range profile.Controls.Items {
		controlResult := e.executeControl(ctx, ctrl)
		result.AddControlResult(controlResult)
	}

	// Finalize result (calculate summary, set end time)
	result.Finalize()

	return result, nil
}

// executeControl executes a single control and returns its result.
func (e *Engine) executeControl(ctx context.Context, ctrl config.Control) ControlResult {
	startTime := time.Now()

	result := ControlResult{
		ID:           ctrl.ID,
		Name:         ctrl.Name,
		Description:  ctrl.Description,
		Severity:     ctrl.Severity,
		Tags:         ctrl.Tags,
		Observations: make([]ObservationResult, 0, len(ctrl.Observations)),
	}

	// Execute each observation
	for _, obs := range ctrl.Observations {
		obsResult := e.executor.Execute(ctx, obs)
		result.Observations = append(result.Observations, obsResult)
	}

	// Aggregate observation results to determine control status
	result.Status = aggregateControlStatus(result.Observations)

	// Generate message based on status
	result.Message = generateControlMessage(result.Status, result.Observations)

	// Set duration
	result.Duration = time.Since(startTime)

	return result
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

	default:
		return "Unknown status"
	}
}

// Close closes the engine and releases resources.
func (e *Engine) Close() error {
	return e.runtime.Close()
}
