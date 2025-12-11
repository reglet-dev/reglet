// Package services contains domain services that encapsulate business logic
// spanning multiple entities. These services are stateless and can be called
// from engine, executor, or future workers.
package services

import (
	"context"
	"fmt"

	"github.com/expr-lang/expr"
	"github.com/whiskeyjimbo/reglet/internal/engine"
	"github.com/whiskeyjimbo/reglet/internal/wasm"
)

// StatusAggregator determines status at different levels of the execution hierarchy
type StatusAggregator struct{}

// NewStatusAggregator creates a new status aggregator service
func NewStatusAggregator() *StatusAggregator {
	return &StatusAggregator{}
}

// AggregateControlStatus determines control status from observation results.
//
// Business Rule: Failure precedence for compliance reporting
// - If ANY observation is StatusFail → Control is StatusFail (proven non-compliance)
// - If ANY observation is StatusError (but no failures) → Control is StatusError (inconclusive)
// - If ALL observations are StatusPass → Control is StatusPass
//
// Rationale: If 9 observations FAIL and 1 errors, the control FAILED (not errored).
// A proven compliance violation is more important than a technical error.
// Auditors need to see definitive failures, not have them masked by errors.
func (s *StatusAggregator) AggregateControlStatus(observations []engine.ObservationResult) engine.Status {
	if len(observations) == 0 {
		return engine.StatusSkipped
	}

	hasFailure := false
	hasError := false

	for _, obs := range observations {
		switch obs.Status {
		case engine.StatusFail:
			hasFailure = true
		case engine.StatusError:
			hasError = true
		case engine.StatusSkipped:
			// Skipped observations don't affect control status
			continue
		case engine.StatusPass:
			// Continue checking other observations
			continue
		}
	}

	// Precedence: Fail > Error > Pass
	if hasFailure {
		return engine.StatusFail
	}
	if hasError {
		return engine.StatusError
	}

	// All observations passed (or were skipped)
	allSkipped := true
	for _, obs := range observations {
		if obs.Status != engine.StatusSkipped {
			allSkipped = false
			break
		}
	}

	if allSkipped {
		return engine.StatusSkipped
	}

	return engine.StatusPass
}

// DetermineObservationStatus evaluates expect expressions against evidence data.
//
// Evaluation Rules:
// - ALL expect expressions must evaluate to true for observation to PASS
// - ANY false expression → observation FAILS
// - Non-boolean result or compilation error → observation ERRORS
//
// Returns: Status and optional error message
func (s *StatusAggregator) DetermineObservationStatus(
	ctx context.Context,
	evidence *wasm.Evidence,
	expects []string,
) (engine.Status, string) {
	// No expectations → use evidence status directly
	if len(expects) == 0 {
		return s.StatusFromEvidenceStatus(evidence.Status), ""
	}

	// Evidence failed with error → don't evaluate expects
	if evidence.Error != nil {
		return engine.StatusError, evidence.Error.Message
	}

	// Evaluate each expect expression
	for _, expectExpr := range expects {
		program, err := expr.Compile(expectExpr, expr.Env(evidence.Data), expr.AsBool())
		if err != nil {
			return engine.StatusError, fmt.Sprintf("expect compilation failed: %v", err)
		}

		output, err := expr.Run(program, evidence.Data)
		if err != nil {
			return engine.StatusError, fmt.Sprintf("expect evaluation failed: %v", err)
		}

		result, ok := output.(bool)
		if !ok {
			return engine.StatusError, fmt.Sprintf("expect expression did not return boolean: %v", output)
		}

		// ANY false expression fails the observation
		if !result {
			return engine.StatusFail, fmt.Sprintf("expectation failed: %s", expectExpr)
		}
	}

	// All expectations passed
	return engine.StatusPass, ""
}

// StatusFromEvidenceStatus converts evidence boolean status to observation status
func (s *StatusAggregator) StatusFromEvidenceStatus(evidenceStatus bool) engine.Status {
	if evidenceStatus {
		return engine.StatusPass
	}
	return engine.StatusFail
}
