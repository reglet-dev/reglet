package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
	"github.com/whiskeyjimbo/reglet/internal/domain/execution"
	"github.com/whiskeyjimbo/reglet/internal/domain/services"
	"github.com/whiskeyjimbo/reglet/internal/domain/values"
	"golang.org/x/sync/errgroup"
)

// executeControl executes a single control and returns its result.
// The index parameter tracks the control's original definition order for deterministic output.
func (e *Engine) executeControl(ctx context.Context, ctrl entities.Control, index int, execResult *execution.ExecutionResult, requiredDeps map[string]bool) execution.ControlResult {
	startTime := time.Now()

	result := execution.ControlResult{
		Index:              index,
		ID:                 ctrl.ID,
		Name:               ctrl.Name,
		Description:        ctrl.Description,
		Severity:           ctrl.Severity,
		Tags:               ctrl.Tags,
		ObservationResults: make([]execution.ObservationResult, 0, len(ctrl.ObservationDefinitions)),
	}

	// Check if control should run (filtering)
	shouldRun, skipReason := e.shouldRun(ctrl)

	// If filtering says skip, check if it's required as a dependency
	if !shouldRun && e.config.IncludeDependencies && requiredDeps[ctrl.ID] {
		shouldRun = true
		skipReason = ""
	}

	if !shouldRun {
		result.Status = values.StatusSkipped
		result.SkipReason = skipReason
		result.Message = skipReason
		result.Duration = time.Since(startTime)
		return result
	}

	// Check dependencies before execution
	if len(ctrl.DependsOn) > 0 {
		for _, depID := range ctrl.DependsOn {
			depStatus, found := execResult.GetControlStatus(depID)

			if !found || depStatus == values.StatusFail || depStatus == values.StatusError || depStatus == values.StatusSkipped {
				result.Status = values.StatusSkipped
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
	if e.config.Parallel && len(ctrl.ObservationDefinitions) > 1 {
		result.ObservationResults = e.executeObservationsParallel(ctx, ctrl.ObservationDefinitions)
	} else {
		for _, obs := range ctrl.ObservationDefinitions {
			obsResult := e.executor.Execute(ctx, obs)
			result.ObservationResults = append(result.ObservationResults, obsResult)
		}
	}

	// Aggregate observation results to determine control status
	observationStatuses := make([]values.Status, len(result.ObservationResults))
	for i, obs := range result.ObservationResults {
		observationStatuses[i] = obs.Status
	}

	aggregator := services.NewStatusAggregator()
	result.Status = aggregator.AggregateControlStatus(observationStatuses)

	// Generate message based on status
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
		g.Go(func() error {
			obsResult := e.executor.Execute(ctx, obs)
			results[i] = obsResult
			return nil
		})
	}

	_ = g.Wait()

	return results
}
