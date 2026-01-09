// Package services contains domain services that encapsulate business logic
// spanning multiple entities. These services are stateless and can be called
// from engine, executor, or future workers.
package services

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/reglet-dev/reglet/internal/domain/execution"
	"github.com/reglet-dev/reglet/internal/domain/values"
)

// StatusAggregator determines status at different levels of the execution hierarchy.
// It caches compiled expressions to avoid redundant compilation overhead.
type StatusAggregator struct {
	programCache map[string]*vm.Program // Cache of compiled expressions (thread-safe with mutex)
	cacheMu      sync.RWMutex           // Protects programCache
}

// NewStatusAggregator creates a new status aggregator service with initialized cache.
func NewStatusAggregator() *StatusAggregator {
	return &StatusAggregator{
		programCache: make(map[string]*vm.Program),
	}
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

// getOrCompileExpression retrieves a cached program or compiles and caches a new one.
// Thread-safe via RWMutex: multiple readers or single writer.
func (s *StatusAggregator) getOrCompileExpression(expression string, options []expr.Option) (*vm.Program, error) {
	// Try read lock first (optimistic path - expression likely cached)
	s.cacheMu.RLock()
	program, found := s.programCache[expression]
	s.cacheMu.RUnlock()

	if found {
		return program, nil
	}

	// Not in cache - compile with write lock
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	// Double-check after acquiring write lock (another goroutine may have compiled it)
	if program, found := s.programCache[expression]; found {
		return program, nil
	}

	// Compile and cache
	program, err := expr.Compile(expression, options...)
	if err != nil {
		return nil, err
	}

	s.programCache[expression] = program
	return program, nil
}

// DetermineObservationStatus evaluates expect expressions against evidence data.
//
// Evaluation Rules:
// - ALL expect expressions must evaluate to true for observation to PASS
// - ANY false expression → observation FAILS
// - Non-boolean result or compilation error → observation ERRORS
//
// Security:
// - Expression length limited to 1000 chars (DoS prevention)
// - Only explicitly provided variables accessible (no probing)
// - expr-lang prevents code execution, filesystem, network access
//
// Performance:
// - Compiled expressions are cached to avoid redundant compilation
// - Thread-safe caching with read/write locks for concurrent execution
//
// Returns: Status and list of expectation results
func (s *StatusAggregator) DetermineObservationStatus(
	_ context.Context,
	evidence *execution.Evidence,
	expects []string,
) (values.Status, []execution.ExpectationResult) {
	// No expectations → use evidence status directly
	if len(expects) == 0 {
		return s.StatusFromEvidenceStatus(evidence.Status), nil
	}

	// Evidence failed with error → don't evaluate expects
	if evidence.Error != nil {
		return values.StatusError, nil
	}

	// Create environment for expression evaluation
	// The evidence data is available under "data" namespace, plus top-level fields
	env := map[string]interface{}{
		"data":      evidence.Data,      // The original data map
		"status":    evidence.Status,    // Top-level status
		"timestamp": evidence.Timestamp, // Top-level timestamp
		"error":     evidence.Error,     // Top-level error
	}

	// Security: Complexity limits to prevent DoS attacks
	const maxExpressionLength = 1000 // Character limit for readability
	const maxASTNodes = 100          // AST node limit prevents deeply nested expressions

	options := []expr.Option{
		expr.Env(env),
		expr.AsBool(),
		expr.MaxNodes(maxASTNodes), // Security: Limit expression complexity (prevents DoS via nested operations)

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

	// Track all expectation results
	results := make([]execution.ExpectationResult, 0, len(expects))
	finalStatus := values.StatusPass

	// Evaluate each expect expression
	for _, expectExpr := range expects {
		// Security: Reject overly long expressions for readability and DoS prevention
		// MaxNodes provides the primary DoS protection via AST complexity limiting
		if len(expectExpr) > maxExpressionLength {
			results = append(results, execution.ExpectationResult{
				Expression: expectExpr,
				Passed:     false,
				Message:    fmt.Sprintf("Expression too long (max %d chars): %d chars", maxExpressionLength, len(expectExpr)),
			})
			finalStatus = values.StatusError
			continue
		}

		// Get or compile expression (uses cache for performance)
		program, err := s.getOrCompileExpression(expectExpr, options)
		if err != nil {
			results = append(results, execution.ExpectationResult{
				Expression: expectExpr,
				Passed:     false,
				Message:    fmt.Sprintf("Compilation failed: %v", err),
			})
			finalStatus = values.StatusError
			continue
		}

		output, err := expr.Run(program, env)
		if err != nil {
			results = append(results, execution.ExpectationResult{
				Expression: expectExpr,
				Passed:     false,
				Message:    fmt.Sprintf("Evaluation failed: %v", err),
			})
			finalStatus = values.StatusError
			continue
		}

		result, ok := output.(bool)
		if !ok {
			results = append(results, execution.ExpectationResult{
				Expression: expectExpr,
				Passed:     false,
				Message:    fmt.Sprintf("Expression did not return boolean: %v", output),
			})
			finalStatus = values.StatusError
			continue
		}

		// Track result
		if result {
			// Expectation passed - no message needed
			results = append(results, execution.ExpectationResult{
				Expression: expectExpr,
				Passed:     true,
			})
		} else {
			// Expectation failed - construct helpful message
			message := s.constructFailureMessage(expectExpr, evidence.Data)
			results = append(results, execution.ExpectationResult{
				Expression: expectExpr,
				Passed:     false,
				Message:    message,
			})
			// Update final status to fail if not already error
			if finalStatus != values.StatusError {
				finalStatus = values.StatusFail
			}
		}
	}

	return finalStatus, results
}

// constructFailureMessage attempts to construct a helpful failure message
// by extracting actual values from evidence data when possible.
func (s *StatusAggregator) constructFailureMessage(expression string, evidenceData map[string]interface{}) string {
	// Try to parse simple comparison expressions like "data.size == 2785"
	// Common patterns: ==, !=, >, <, >=, <=
	patterns := []string{"==", "!=", ">=", "<=", ">", "<"}

	for _, op := range patterns {
		if strings.Contains(expression, op) {
			parts := strings.SplitN(expression, op, 2)
			if len(parts) == 2 {
				left := strings.TrimSpace(parts[0])
				right := strings.TrimSpace(parts[1])

				// Try to extract actual value if left side is a data reference
				if strings.HasPrefix(left, "data.") {
					fieldPath := strings.TrimPrefix(left, "data.")
					if actualValue, ok := evidenceData[fieldPath]; ok {
						return fmt.Sprintf("Expected %s %s %s, got %v", left, op, right, actualValue)
					}
				}
			}
		}
	}

	// Fallback: just state the expression failed
	return fmt.Sprintf("Expression evaluated to false: %s", expression)
}

// StatusFromEvidenceStatus converts evidence boolean status to observation status
func (s *StatusAggregator) StatusFromEvidenceStatus(evidenceStatus bool) values.Status {
	if evidenceStatus {
		return values.StatusPass
	}
	return values.StatusFail
}
