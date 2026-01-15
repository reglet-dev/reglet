package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewExpectValidator_Defaults(t *testing.T) {
	t.Parallel()

	v := NewExpectValidator()

	require.NotNil(t, v)
	assert.Equal(t, 1000, v.maxExpressionLength)
	assert.Equal(t, 100, v.maxASTNodes)
}

func TestNewExpectValidator_WithOptions(t *testing.T) {
	t.Parallel()

	v := NewExpectValidator(WithExpectLimits(500, 50))

	require.NotNil(t, v)
	assert.Equal(t, 500, v.maxExpressionLength)
	assert.Equal(t, 50, v.maxASTNodes)
}

func TestExpectValidator_ValidateExpression_Valid(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		expression string
	}{
		{"simple_comparison", "data.exists == true"},
		{"numeric_comparison", "data.size > 100"},
		{"string_contains", `"foo" in data.content`},
		{"logical_and", "data.exists && data.readable"},
		{"logical_or", "data.size > 0 || data.empty"},
		{"not_operator", "!data.sensitive"},
		{"nested_field", "data.file.permissions == 0644"},
		{"status_check", "status == true"},
		{"function_call", "len(data.lines) > 5"},
		{"array_access", "data.items[0] == 'test'"},
		{"ternary", "data.exists ? data.size > 0 : false"},
	}

	v := NewExpectValidator()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := v.ValidateExpression(tc.expression)
			assert.NoError(t, err)
		})
	}
}

func TestExpectValidator_ValidateExpression_Invalid(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		expression  string
		expectedErr string
	}{
		{"empty", "", "expression cannot be empty"},
		{"unclosed_paren", "(data.exists", "syntax error"},
		{"invalid_operator", "data.size >< 100", "syntax error"},
		{"unclosed_string", `data.name == "foo`, "syntax error"},
		{"missing_operand", "data.exists &&", "syntax error"},
		{"invalid_syntax", "data..size", "syntax error"},
	}

	v := NewExpectValidator()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := v.ValidateExpression(tc.expression)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}

func TestExpectValidator_ValidateExpression_TooLong(t *testing.T) {
	t.Parallel()

	v := NewExpectValidator(WithExpectLimits(50, 100))

	// Create expression longer than 50 chars
	longExpr := "data.very_long_field_name_that_exceeds_limit == true"
	require.Greater(t, len(longExpr), 50)

	err := v.ValidateExpression(longExpr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expression too long")
}

func TestExpectValidator_ValidateObservationExpects(t *testing.T) {
	t.Parallel()

	v := NewExpectValidator()

	t.Run("all_valid", func(t *testing.T) {
		t.Parallel()
		expects := []string{
			"data.exists == true",
			"data.size > 0",
		}
		errs := v.ValidateObservationExpects(expects)
		assert.Empty(t, errs)
	})

	t.Run("some_invalid", func(t *testing.T) {
		t.Parallel()
		expects := []string{
			"data.exists == true",
			"(invalid syntax",
			"data.size > 0",
			"also invalid &&",
		}
		errs := v.ValidateObservationExpects(expects)
		assert.Len(t, errs, 2)
		assert.Equal(t, "(invalid syntax", errs[0].Expression)
		assert.Equal(t, "also invalid &&", errs[1].Expression)
	})

	t.Run("empty_expects", func(t *testing.T) {
		t.Parallel()
		errs := v.ValidateObservationExpects(nil)
		assert.Nil(t, errs)
	})
}

func TestExpectValidator_ValidateProfileExpects(t *testing.T) {
	t.Parallel()

	v := NewExpectValidator()

	controls := []struct {
		ID           string
		Observations []struct {
			Expects []string
		}
	}{
		{
			ID: "ctrl-1",
			Observations: []struct{ Expects []string }{
				{Expects: []string{"data.exists"}},
				{Expects: []string{"(invalid"}}, // Invalid
			},
		},
		{
			ID: "ctrl-2",
			Observations: []struct{ Expects []string }{
				{Expects: []string{"data.size > 0"}}, // All valid
			},
		},
		{
			ID: "ctrl-3",
			Observations: []struct{ Expects []string }{
				{Expects: []string{"bad &&"}}, // Invalid
			},
		},
	}

	result := v.ValidateProfileExpects(controls)

	// ctrl-1 has errors in observation 1
	require.Contains(t, result, "ctrl-1")
	assert.Contains(t, result["ctrl-1"], 1)
	assert.NotContains(t, result["ctrl-1"], 0)

	// ctrl-2 has no errors
	assert.NotContains(t, result, "ctrl-2")

	// ctrl-3 has errors in observation 0
	require.Contains(t, result, "ctrl-3")
	assert.Contains(t, result["ctrl-3"], 0)
}

func TestExpectValidationError_Error(t *testing.T) {
	t.Parallel()

	err := ExpectValidationError{
		Expression: "data.exists &&",
		Message:    "syntax error: unexpected end of expression",
	}

	assert.Contains(t, err.Error(), "data.exists &&")
	assert.Contains(t, err.Error(), "syntax error")
}
