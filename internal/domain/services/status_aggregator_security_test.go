package services

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/whiskeyjimbo/reglet/internal/domain/execution"
	"github.com/whiskeyjimbo/reglet/internal/domain/values"
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
			name: "safe helper function usage",
			expects: []string{
				"strContains(data.name, 'test')",
			},
			wantStatus: values.StatusPass,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, errMsg := aggregator.DetermineObservationStatus(ctx, evidence, tt.expects)
			assert.Equal(t, tt.wantStatus, status)

			if tt.wantErrType != "" {
				require.NotEmpty(t, errMsg, "expected error message")
				switch tt.wantErrType {
				case "compilation":
					assert.Contains(t, errMsg, "compilation failed", "error should indicate compilation failure")
				case "evaluation":
					assert.Contains(t, errMsg, "evaluation failed", "error should indicate evaluation failure")
				case "length":
					assert.Contains(t, errMsg, "too long", "error should indicate length limit")
				case "expectation":
					assert.Contains(t, errMsg, "expectation failed", "error should indicate expectation failure")
				}
			} else {
				assert.Empty(t, errMsg, "unexpected error: %s", errMsg)
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
			status, errMsg := aggregator.DetermineObservationStatus(ctx, evidence, []string{expr})
			assert.Equal(t, values.StatusError, status, "malicious expression should fail")
			assert.NotEmpty(t, errMsg, "should have error message")
			assert.Contains(t, errMsg, "compilation failed", "should fail at compilation")
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
			name:       "strContains valid",
			expr:       "strContains(data.hostname, 'example')",
			wantStatus: values.StatusPass,
		},
		{
			name:       "strContains invalid",
			expr:       "strContains(data.hostname, 'evil')",
			wantStatus: values.StatusFail,
		},
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
		{
			name:       "strContains wrong argument count",
			expr:       "strContains(data.hostname)",
			wantStatus: values.StatusError,
		},
		{
			name:       "strContains wrong type",
			expr:       "strContains(123, 'test')",
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
