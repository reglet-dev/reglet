// Package services contains domain services that encapsulate business logic
// spanning multiple entities. These services are stateless and can be called
// from engine, executor, or future workers.
package services

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/whiskeyjimbo/reglet/internal/domain/execution"
	"github.com/whiskeyjimbo/reglet/internal/domain/values"
)

// StatusAggregator determines status at different levels of the execution hierarchy
type StatusAggregator struct{}

// NewStatusAggregator creates a new status aggregator service
func NewStatusAggregator() *StatusAggregator {
	return &StatusAggregator{}
}

// AggregateControlStatus determines control status from observation statuses.
//
// Business Rule: Failure precedence for compliance reporting
// - If ANY observation is StatusFail → Control is StatusFail (proven non-compliance)
// - If ANY observation is StatusError (but no failures) → Control is StatusError (inconclusive)
// - If ALL observations are StatusPass → Control is StatusPass
//
// Rationale: If 9 observations FAIL and 1 errors, the control FAILED (not errored).
// A proven compliance violation is more important than a technical error.
// Auditors need to see definitive failures, not have them masked by errors.
func (s *StatusAggregator) AggregateControlStatus(observationStatuses []values.Status) values.Status {
	if len(observationStatuses) == 0 {
		return values.StatusSkipped
	}

	hasFailure := false
	hasError := false

	for _, status := range observationStatuses {
		switch status {
		case values.StatusFail:
			hasFailure = true
		case values.StatusError:
			hasError = true
		case values.StatusSkipped:
			// Skipped observations don't affect control status
			continue
		case values.StatusPass:
			// Continue checking other observations
			continue
		}
	}

	// Precedence: Fail > Error > Pass
	if hasFailure {
		return values.StatusFail
	}
	if hasError {
		return values.StatusError
	}

	// All observations passed (or were skipped)
	allSkipped := true
	for _, status := range observationStatuses {
		if status != values.StatusSkipped {
			allSkipped = false
			break
		}
	}

	if allSkipped {
		return values.StatusSkipped
	}

	return values.StatusPass
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
	evidence *execution.Evidence,
	expects []string,
) (values.Status, string) {
	// No expectations → use evidence status directly
	if len(expects) == 0 {
		return s.StatusFromEvidenceStatus(evidence.Status), ""
	}

	// Evidence failed with error → don't evaluate expects
	if evidence.Error != nil {
		return values.StatusError, evidence.Error.Message
	}

	// Create environment for expression evaluation
	// The evidence data is available under "data" namespace, plus top-level fields
	env := map[string]interface{}{
		"data":      evidence.Data,      // The original data map
		"status":    evidence.Status,    // Top-level status
		"timestamp": evidence.Timestamp, // Top-level timestamp
		"error":     evidence.Error,     // Top-level error
	}

	options := []expr.Option{
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

	// Evaluate each expect expression
	for _, expectExpr := range expects {
		program, err := expr.Compile(expectExpr, options...)
		if err != nil {
			return values.StatusError, fmt.Sprintf("expect compilation failed: %v", err)
		}

		output, err := expr.Run(program, env)
		if err != nil {
			return values.StatusError, fmt.Sprintf("expect evaluation failed: %v", err)
		}

		result, ok := output.(bool)
		if !ok {
			return values.StatusError, fmt.Sprintf("expect expression did not return boolean: %v", output)
		}

		// ANY false expression fails the observation
		if !result {
			return values.StatusFail, fmt.Sprintf("expectation failed: %s", expectExpr)
		}
	}

	// All expectations passed
	return values.StatusPass, ""
}

// StatusFromEvidenceStatus converts evidence boolean status to observation status
func (s *StatusAggregator) StatusFromEvidenceStatus(evidenceStatus bool) values.Status {
	if evidenceStatus {
		return values.StatusPass
	}
	return values.StatusFail
}
