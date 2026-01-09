package services

import (
	"context"
	"strings"
	"testing"

	"github.com/reglet-dev/reglet/internal/domain/execution"
	"github.com/reglet-dev/reglet/internal/domain/values"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDetermineObservationStatus_SecurityHardening verifies expr expression security
func TestDetermineObservationStatus_SecurityHardening(t *testing.T) {
	aggregator := NewStatusAggregator()
	ctx := context.Background()

	evidence := &execution.Evidence{
		Status: true,
		Data: map[string]interface{}{
			"count": 42,
			"name":  "test",
		},
	}

	tests := []struct {
		name        string
		expects     []string
		wantStatus  values.Status
		wantErrType string // "compilation", "evaluation", "length", or ""
	}{
		{
			name:       "legitimate expression",
			expects:    []string{"data.count > 10"},
			wantStatus: values.StatusPass,
		},
		{
			name:       "legitimate complex expression",
			expects:    []string{"data.count > 10 && data.count < 100 && data.name == 'test'"},
			wantStatus: values.StatusPass,
		},
		{
			name: "expression too long (DoS prevention)",
			expects: []string{
				strings.Repeat("data.count > 0 && ", 100) + "true", // >1000 chars
			},
			wantStatus:  values.StatusError,
			wantErrType: "length",
		},
		{
			name: "deeply nested expression (AST node limit)",
			expects: []string{
				// Construct expression with >100 AST nodes
				"(" + strings.Repeat("1+", 150) + "1) > 0",
			},
			wantStatus:  values.StatusError,
			wantErrType: "compilation", // MaxNodes should prevent compilation
		},
		{
			name: "undefined variable access (security)",
			expects: []string{
				"undefinedVar == 'test'",
			},
			wantStatus:  values.StatusError,
			wantErrType: "compilation", // Default expr behavior disallows undefined vars
		},
		{
			name:        "multiple legitimate expressions",
			expects:     []string{"data.count > 10", "data.name == 'test'"},
			wantStatus:  values.StatusPass,
			wantErrType: "",
		},
		{
			name: "expression fails expectation (not security issue)",
			expects: []string{
				"data.count > 100", // Fails because count is 42
			},
			wantStatus:  values.StatusFail,
			wantErrType: "expectation", // Failing expectation returns error message
		},
		{
			name: "safe native operator usage",
			expects: []string{
				"data.name contains 'test'",
			},
			wantStatus: values.StatusPass,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, results := aggregator.DetermineObservationStatus(ctx, evidence, tt.expects)
			assert.Equal(t, tt.wantStatus, status)

			if tt.wantErrType != "" {
				// Should have at least one failed expectation
				require.NotEmpty(t, results, "expected expectation results")

				// Find a failed expectation with the right error type
				foundError := false
				for _, result := range results {
					if !result.Passed && result.Message != "" {
						switch tt.wantErrType {
						case "compilation":
							if strings.Contains(result.Message, "Compilation failed") {
								foundError = true
							}
						case "evaluation":
							if strings.Contains(result.Message, "Evaluation failed") {
								foundError = true
							}
						case "length":
							if strings.Contains(result.Message, "too long") {
								foundError = true
							}
						case "expectation":
							// Any failed expectation with a message counts
							foundError = true
						}
					}
				}
				assert.True(t, foundError, "expected error type %q not found in results", tt.wantErrType)
			} else {
				// All expectations should pass
				for _, result := range results {
					assert.True(t, result.Passed, "expected all expectations to pass")
				}
			}
		})
	}
}

// TestDetermineObservationStatus_NoCodeExecution verifies expr cannot execute code
func TestDetermineObservationStatus_NoCodeExecution(t *testing.T) {
	aggregator := NewStatusAggregator()
	ctx := context.Background()

	evidence := &execution.Evidence{
		Status: true,
		Data: map[string]interface{}{
			"value": "test",
		},
	}

	// expr-lang fundamentally does NOT support:
	// - Arbitrary code execution
	// - File system access
	// - Network access
	// - OS command execution
	// - Package imports
	// - Goroutine creation
	//
	// These would all fail at compilation (not even evaluation)

	maliciousExpressions := []string{
		// These are syntactically valid but will fail due to undefined functions/variables
		"exec('ls')",               // No exec function
		"import('os')",             // No import support
		"File.Read('/etc/passwd')", // No File object
		"HTTP.Get('evil.com')",     // No HTTP object
		"os.Getenv('AWS_KEY')",     // No os package
		"goroutine { data.value }", // No goroutine syntax
	}

	for _, expr := range maliciousExpressions {
		t.Run(expr, func(t *testing.T) {
			status, results := aggregator.DetermineObservationStatus(ctx, evidence, []string{expr})
			assert.Equal(t, values.StatusError, status, "malicious expression should fail")
			require.NotEmpty(t, results, "should have expectation results")
			require.False(t, results[0].Passed, "expectation should fail")
			assert.Contains(t, results[0].Message, "Compilation failed", "should fail at compilation")
		})
	}
}

// TestDetermineObservationStatus_SafeEnvironment verifies only expected vars are accessible
func TestDetermineObservationStatus_SafeEnvironment(t *testing.T) {
	aggregator := NewStatusAggregator()
	ctx := context.Background()

	evidence := &execution.Evidence{
		Status: true,
		Data: map[string]interface{}{
			"count": 42,
		},
	}

	tests := []struct {
		name       string
		expr       string
		wantStatus values.Status
	}{
		{
			name:       "access data",
			expr:       "data.count > 0",
			wantStatus: values.StatusPass,
		},
		{
			name:       "access status",
			expr:       "status == true",
			wantStatus: values.StatusPass,
		},
		{
			name:       "access timestamp",
			expr:       "timestamp != nil || true", // timestamp may be zero value
			wantStatus: values.StatusPass,
		},
		{
			name:       "undefined variable fails",
			expr:       "undefinedVar > 0",
			wantStatus: values.StatusError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, _ := aggregator.DetermineObservationStatus(ctx, evidence, []string{tt.expr})
			assert.Equal(t, tt.wantStatus, status)
		})
	}
}

// TestDetermineObservationStatus_HelperFunctions verifies safe custom functions
func TestDetermineObservationStatus_HelperFunctions(t *testing.T) {
	aggregator := NewStatusAggregator()
	ctx := context.Background()

	evidence := &execution.Evidence{
		Status: true,
		Data: map[string]interface{}{
			"hostname": "server.example.com",
			"ip":       "192.168.1.1",
		},
	}

	tests := []struct {
		name       string
		expr       string
		wantStatus values.Status
	}{
		{
			name:       "isIPv4 valid",
			expr:       "isIPv4(data.ip)",
			wantStatus: values.StatusPass,
		},
		{
			name:       "isIPv4 invalid",
			expr:       "isIPv4(data.hostname)",
			wantStatus: values.StatusFail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, _ := aggregator.DetermineObservationStatus(ctx, evidence, []string{tt.expr})
			assert.Equal(t, tt.wantStatus, status)
		})
	}
}
