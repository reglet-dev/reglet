package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/execution"
	"github.com/reglet-dev/reglet/internal/domain/values"
	"github.com/reglet-dev/reglet/internal/pkg/loopexpander"
)

// executeLoopObservation expands and executes a loop observation.
// It resolves the loop items from the profile vars, executes each child observation,
// and aggregates the results.
func (e *Engine) executeLoopObservation(ctx context.Context, obs entities.ObservationDefinition, vars map[string]interface{}) execution.ObservationResult {
	startTime := time.Now()

	// Resolve the items from the loop expression
	items, err := loopexpander.ResolveLoopItems(obs.Loop.Items, vars)
	if err != nil {
		return execution.ObservationResult{
			Plugin:   obs.Plugin,
			Status:   values.StatusError,
			RawError: fmt.Errorf("loop items: %w", err),
			IsLoop:   true,
			Duration: time.Since(startTime),
		}
	}

	if len(items) == 0 {
		return execution.ObservationResult{
			Plugin:   obs.Plugin,
			Status:   values.StatusPass, // Empty loop = pass
			IsLoop:   true,
			Children: []execution.ObservationResult{},
			Duration: time.Since(startTime),
		}
	}

	// Execute each child observation
	children := make([]execution.ObservationResult, len(items))
	for i, item := range items {
		loopCtx := &loopexpander.Context{
			Item:   item,
			Index:  i,
			First:  i == 0,
			Last:   i == len(items)-1,
			Length: len(items),
		}
		childObs := expandLoopObservation(obs, loopCtx)
		children[i] = e.executor.Execute(ctx, childObs)
		children[i].LoopItem = item
		children[i].LoopIndex = i

		// Apply evidence truncation
		if children[i].Evidence != nil && children[i].Evidence.Data != nil {
			limit := e.config.MaxEvidenceSizeBytes
			if limit == 0 {
				limit = execution.DefaultMaxEvidenceSize
			}
			truncated, meta, truncErr := e.truncator.Truncate(children[i].Evidence.Data, limit)
			if truncErr == nil && meta != nil {
				children[i].Evidence.Data = truncated
				children[i].EvidenceMeta = meta
			}
		}
	}

	// Aggregate results (all must pass)
	status := values.StatusPass
	for _, child := range children {
		if child.Status == values.StatusFail || child.Status == values.StatusError {
			status = values.StatusFail
			break
		}
	}

	return execution.ObservationResult{
		Plugin:   obs.Plugin,
		Status:   status,
		IsLoop:   true,
		Children: children,
		Duration: time.Since(startTime),
	}
}

// expandLoopObservation creates a child observation with loop context substituted into config and expect.
func expandLoopObservation(obs entities.ObservationDefinition, loopCtx *loopexpander.Context) entities.ObservationDefinition {
	customName := ""
	if obs.Loop != nil {
		customName = obs.Loop.As
	}

	// Clone and substitute config
	newConfig := loopexpander.SubstituteLoopInMap(obs.Config, loopCtx, customName)

	// Substitute in expect expressions
	newExpect := make([]string, len(obs.Expect))
	for i, expr := range obs.Expect {
		newExpect[i] = loopexpander.SubstituteLoopInString(expr, loopCtx, customName)
	}

	return entities.ObservationDefinition{
		Plugin: obs.Plugin,
		Config: newConfig,
		Expect: newExpect,
		// No Loop - this is an expanded child
	}
}
