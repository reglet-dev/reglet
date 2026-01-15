// Package services contains domain services that encapsulate business logic
// spanning multiple entities.
package services

import (
	"fmt"

	"github.com/expr-lang/expr"
	"github.com/reglet-dev/reglet/internal/domain/constants"
)

// ExpectValidationError represents a validation failure for an expect expression.
type ExpectValidationError struct {
	Expression string
	Message    string
}

// Error implements the error interface.
func (e ExpectValidationError) Error() string {
	return fmt.Sprintf("expect expression %q: %s", e.Expression, e.Message)
}

// ExpectValidator validates expr-lang expect expressions without execution.
// This enables pre-flight validation during profile validation, catching syntax
// errors before attempting to run controls.
type ExpectValidator struct {
	maxExpressionLength int
	maxASTNodes         int
}

// ExpectValidatorOption defines functional options for ExpectValidator.
type ExpectValidatorOption func(*ExpectValidator)

// WithExpectLimits sets custom expression limits.
func WithExpectLimits(maxLength, maxNodes int) ExpectValidatorOption {
	return func(v *ExpectValidator) {
		if maxLength > 0 {
			v.maxExpressionLength = maxLength
		}
		if maxNodes > 0 {
			v.maxASTNodes = maxNodes
		}
	}
}

// NewExpectValidator creates a new expect validator with configurable limits.
func NewExpectValidator(opts ...ExpectValidatorOption) *ExpectValidator {
	v := &ExpectValidator{
		maxExpressionLength: constants.DefaultMaxExpressionLength,
		maxASTNodes:         constants.DefaultMaxASTNodes,
	}

	for _, opt := range opts {
		opt(v)
	}

	return v
}

// ValidateExpression validates a single expect expression.
// Returns nil if valid, error describing syntax issue if invalid.
//
// Note: This validates syntax only. Since expect expressions reference
// evidence data (e.g., "data.exists == true"), we cannot validate against
// a real environment. We use a permissive environment that accepts any field access.
func (v *ExpectValidator) ValidateExpression(expression string) error {
	// Check expression length
	if len(expression) > v.maxExpressionLength {
		return fmt.Errorf("expression too long (%d chars, max %d)",
			len(expression), v.maxExpressionLength)
	}

	if expression == "" {
		return fmt.Errorf("expression cannot be empty")
	}

	// Convert maxASTNodes with overflow check
	maxNodes := uint(v.maxASTNodes)
	if v.maxASTNodes < 0 {
		maxNodes = uint(constants.DefaultMaxASTNodes)
	}

	// Create a permissive environment for syntax validation.
	// We can't know the actual evidence structure at validation time,
	// so we use expr.AllowUndefinedVariables() to accept any field access.
	options := []expr.Option{
		expr.AsBool(),
		expr.MaxNodes(maxNodes),
		expr.AllowUndefinedVariables(), // Allow any variable access during syntax check
	}

	// Compile the expression to check for syntax errors
	_, err := expr.Compile(expression, options...)
	if err != nil {
		return fmt.Errorf("syntax error: %w", err)
	}

	return nil
}

// ValidateObservationExpects validates all expect expressions in an observation.
// Returns a slice of validation errors (empty if all expressions are valid).
func (v *ExpectValidator) ValidateObservationExpects(expects []string) []ExpectValidationError {
	if len(expects) == 0 {
		return nil
	}

	var errors []ExpectValidationError
	for _, expectExpr := range expects {
		if err := v.ValidateExpression(expectExpr); err != nil {
			errors = append(errors, ExpectValidationError{
				Expression: expectExpr,
				Message:    err.Error(),
			})
		}
	}

	return errors
}

// ValidateProfileExpects validates all expect expressions across all controls
// in a profile. Returns a map of control ID -> observation index -> errors.
// This provides comprehensive validation results for display purposes.
func (v *ExpectValidator) ValidateProfileExpects(
	controls []struct {
		ID           string
		Observations []struct {
			Expects []string
		}
	},
) map[string]map[int][]ExpectValidationError {
	result := make(map[string]map[int][]ExpectValidationError)

	for _, ctrl := range controls {
		controlErrors := make(map[int][]ExpectValidationError)
		hasErrors := false

		for obsIdx, obs := range ctrl.Observations {
			if errs := v.ValidateObservationExpects(obs.Expects); len(errs) > 0 {
				controlErrors[obsIdx] = errs
				hasErrors = true
			}
		}

		if hasErrors {
			result[ctrl.ID] = controlErrors
		}
	}

	return result
}
