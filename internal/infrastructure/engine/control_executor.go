package engine

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/execution"
	"github.com/reglet-dev/reglet/internal/domain/services"
	"github.com/reglet-dev/reglet/internal/domain/values"
	"golang.org/x/sync/errgroup"
)

// executeControl executes a single control and returns its result.
// The index parameter tracks the control's original definition order for deterministic output.
func (e *Engine) executeControl(ctx context.Context, ctrl entities.Control, index int, execResult *execution.ExecutionResult, requiredDeps map[string]bool) execution.ControlResult {
	startTime := time.Now()
	result := newControlResult(ctrl, index)

	// Check skip conditions
	if skipReason := e.checkSkipConditions(ctrl, execResult, requiredDeps); skipReason != "" {
		return skipControl(result, skipReason, startTime)
	}

	maxAttempts := ctrl.Retries + 1
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Execute observations
		result.ObservationResults = e.runObservations(ctx, ctrl)

		// Aggregate and finalize
		result = finalizeResult(result, startTime)

		// If success or permanent failure, we are done
		if result.Status != values.StatusError {
			return result
		}

		// Check for transient failure
		isTransient := false
		for _, obs := range result.ObservationResults {
			if obs.RawError != nil && isTransientError(obs.RawError) {
				isTransient = true
				lastErr = obs.RawError
				break
			}
		}

		if !isTransient {
			return result
		}

		// If we are here, it is a transient error.
		if attempt < maxAttempts {
			delay := CalculateBackoff(
				ctrl.RetryBackoff,
				attempt,
				ctrl.RetryDelay,
				ctrl.RetryMaxDelay,
			)

			slog.InfoContext(ctx, "Retrying control",
				"control", ctrl.ID,
				"attempt", attempt,
				"max_attempts", maxAttempts,
				"delay", delay,
				"error", lastErr,
			)

			select {
			case <-ctx.Done():
				return result // Context canceled, stop retrying
			case <-time.After(delay):
				// Continue to next attempt
			}
		}
	}

	return result
}

// newControlResult creates an initial ControlResult from a control definition.
func newControlResult(ctrl entities.Control, index int) execution.ControlResult {
	return execution.ControlResult{
		Index:              index,
		ID:                 ctrl.ID,
		Name:               ctrl.Name,
		Description:        ctrl.Description,
		Severity:           ctrl.Severity,
		Tags:               ctrl.Tags,
		ObservationResults: make([]execution.ObservationResult, 0, len(ctrl.ObservationDefinitions)),
	}
}

// checkSkipConditions returns a skip reason if the control should be skipped.
func (e *Engine) checkSkipConditions(ctrl entities.Control, execResult *execution.ExecutionResult, requiredDeps map[string]bool) string {
	shouldRun, skipReason := e.shouldRun(ctrl)

	// If filtering says skip, check if it's required as a dependency
	if !shouldRun && e.config.IncludeDependencies && requiredDeps[ctrl.ID] {
		shouldRun = true
		skipReason = ""
	}

	if !shouldRun {
		return skipReason
	}

	// Check dependencies
	return e.checkDependencies(ctrl, execResult)
}

// checkDependencies verifies all dependencies have passed.
func (e *Engine) checkDependencies(ctrl entities.Control, execResult *execution.ExecutionResult) string {
	for _, depID := range ctrl.DependsOn {
		depStatus, found := execResult.GetControlStatus(depID)
		if !found {
			return fmt.Sprintf("Skipped: dependency '%s' not found", depID)
		}
		if depStatus == values.StatusFail || depStatus == values.StatusError || depStatus == values.StatusSkipped {
			return fmt.Sprintf("Skipped: dependency '%s' has status '%s'", depID, depStatus)
		}
	}
	return ""
}

// skipControl creates a skipped control result.
func skipControl(result execution.ControlResult, skipReason string, startTime time.Time) execution.ControlResult {
	result.Status = values.StatusSkipped
	result.SkipReason = skipReason
	result.Message = skipReason
	result.Duration = time.Since(startTime)
	return result
}

// runObservations executes observations sequentially or in parallel.
func (e *Engine) runObservations(ctx context.Context, ctrl entities.Control) []execution.ObservationResult {
	if e.config.Parallel && len(ctrl.ObservationDefinitions) > 1 {
		return e.executeObservationsParallel(ctx, ctrl.ObservationDefinitions)
	}

	results := make([]execution.ObservationResult, 0, len(ctrl.ObservationDefinitions))
	for _, obs := range ctrl.ObservationDefinitions {
		obsResult := e.executor.Execute(ctx, obs)

		limit := e.config.MaxEvidenceSizeBytes
		if limit == 0 {
			limit = execution.DefaultMaxEvidenceSize
		}

		if obsResult.Evidence != nil && obsResult.Evidence.Data != nil {
			truncated, meta, err := e.truncator.Truncate(obsResult.Evidence.Data, limit)
			if err != nil {
				slog.ErrorContext(ctx, "failed to truncate evidence", "error", err, "plugin", obsResult.Plugin)
			} else if meta != nil {
				obsResult.Evidence.Data = truncated
				obsResult.EvidenceMeta = meta
			}
		}

		results = append(results, obsResult)
	}
	return results
}

// finalizeResult aggregates observation statuses and generates the control message.
func finalizeResult(result execution.ControlResult, startTime time.Time) execution.ControlResult {
	statuses := make([]values.Status, len(result.ObservationResults))
	for i, obs := range result.ObservationResults {
		statuses[i] = obs.Status
	}

	aggregator := services.NewStatusAggregator()
	result.Status = aggregator.AggregateControlStatus(statuses)
	result.Message = generateControlMessage(result.Status, result.ObservationResults)
	result.Duration = time.Since(startTime)

	return result
}

// shouldRun determines if a control should run based on the configuration filters.
func (e *Engine) shouldRun(ctrl entities.Control) (bool, string) {
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
func (e *Engine) executeObservationsParallel(ctx context.Context, observations []entities.ObservationDefinition) []execution.ObservationResult {
	g, ctx := errgroup.WithContext(ctx)

	if e.config.MaxConcurrentObservations > 0 {
		g.SetLimit(e.config.MaxConcurrentObservations)
	}

	results := make([]execution.ObservationResult, len(observations))

	for i, obs := range observations {
		i, obs := i, obs // capture for closure
		g.Go(func() error {
			obsResult := e.executor.Execute(ctx, obs)

			limit := e.config.MaxEvidenceSizeBytes
			if limit == 0 {
				limit = execution.DefaultMaxEvidenceSize
			}

			if obsResult.Evidence != nil && obsResult.Evidence.Data != nil {
				truncated, meta, err := e.truncator.Truncate(obsResult.Evidence.Data, limit)
				if err != nil {
					slog.ErrorContext(ctx, "failed to truncate evidence", "error", err, "plugin", obsResult.Plugin)
				} else if meta != nil {
					obsResult.Evidence.Data = truncated
					obsResult.EvidenceMeta = meta
				}
			}

			results[i] = obsResult
			return nil
		})
	}

	_ = g.Wait()

	return results
}
