package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/whiskeyjimbo/reglet/internal/domain/execution"
	"github.com/whiskeyjimbo/reglet/internal/domain/values"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/wasm"
)

// createTestResult creates a sample execution result for testing.
func createTestResult() *execution.ExecutionResult {
	result := execution.NewExecutionResult("test-profile", "1.0.0")

	// Add a passing control
	passingControl := execution.ControlResult{
		ID:          "ctrl-1",
		Name:        "Test Control 1",
		Description: "A test control that passes",
		Severity:    "high",
		Tags:        []string{"security", "test"},
		Status:      values.StatusPass,
		Message:     "All 2 checks passed",
		Duration:    100 * time.Millisecond,
		Observations: []execution.ObservationResult{
			{
				Plugin: "file",
				Config: map[string]interface{}{
					"path": "/etc/test",
					"mode": "exists",
				},
				Status: values.StatusPass,
				Evidence: &wasm.Evidence{
					Timestamp: time.Now(),
					Data: map[string]interface{}{
						"path":   "/etc/test",
						"exists": true,
						"status": true,
					},
				},
				Duration: 50 * time.Millisecond,
			},
			{
				Plugin: "file",
				Config: map[string]interface{}{
					"path": "/etc/test2",
					"mode": "exists",
				},
				Status: values.StatusPass,
				Evidence: &wasm.Evidence{
					Timestamp: time.Now(),
					Data: map[string]interface{}{
						"path":   "/etc/test2",
						"exists": true,
						"status": true,
					},
				},
				Duration: 50 * time.Millisecond,
			},
		},
	}

	// Add a failing control
	failingControl := execution.ControlResult{
		ID:          "ctrl-2",
		Name:        "Test Control 2",
		Description: "A test control that fails",
		Severity:    "medium",
		Status:      values.StatusFail,
		Message:     "1 check failed",
		Duration:    50 * time.Millisecond,
		Observations: []execution.ObservationResult{
			{
				Plugin: "file",
				Config: map[string]interface{}{
					"path": "/etc/missing",
					"mode": "exists",
				},
				Status: values.StatusFail,
				Evidence: &wasm.Evidence{
					Timestamp: time.Now(),
					Data: map[string]interface{}{
						"path":   "/etc/missing",
						"exists": false,
						"status": false,
					},
				},
				Duration: 50 * time.Millisecond,
			},
		},
	}

	// Add an error control
	errorControl := execution.ControlResult{
		ID:       "ctrl-3",
		Name:     "Test Control 3",
		Severity: "critical",
		Status:   values.StatusError,
		Message:  "Plugin load failed",
		Duration: 10 * time.Millisecond,
		Observations: []execution.ObservationResult{
			{
				Plugin: "nonexistent",
				Config: map[string]interface{}{
					"test": "value",
				},
				Status: values.StatusError,
				Error: &wasm.PluginError{
					Code:    "plugin_load_error",
					Message: "unknown plugin: nonexistent",
				},
				Duration: 10 * time.Millisecond,
			},
		},
	}

	result.AddControlResult(passingControl)
	result.AddControlResult(failingControl)
	result.AddControlResult(errorControl)
	result.Finalize()

	return result
}

func TestTableFormatter_Format(t *testing.T) {
	result := createTestResult()
	var buf bytes.Buffer
	formatter := NewTableFormatter(&buf)
	formatter.EnableColor = false // Disable color for deterministic string comparison

	err := formatter.Format(result)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Profile: test-profile (v1.0.0)")
	assert.Contains(t, output, "ctrl-1: Test Control 1")
	assert.Contains(t, output, "ctrl-2: Test Control 2")
	assert.Contains(t, output, "ctrl-3: Test Control 3")
	assert.Contains(t, output, "Summary:")
	assert.Contains(t, output, "Controls:     3 total")
	assert.Contains(t, output, "Passed:   1")
	assert.Contains(t, output, "Failed:   1")
	assert.Contains(t, output, "Errors:   1")
}

func TestTableFormatter_EmptyResult(t *testing.T) {
	result := createTestResult()
	result.ProfileName = "empty-profile"
	result.Controls = []execution.ControlResult{}

	var buf bytes.Buffer
	formatter := NewTableFormatter(&buf)
	formatter.EnableColor = false // Disable color

	err := formatter.Format(result)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Profile: empty-profile (v1.0.0)")
	assert.Contains(t, output, "No controls executed.")
}

func TestJSONFormatter_Format_Indented(t *testing.T) {
	t.Parallel()
	result := createTestResult()
	var buf bytes.Buffer

	formatter := NewJSONFormatter(&buf, true)
	err := formatter.Format(result)
	require.NoError(t, err)

	output := buf.String()

	// Verify it's valid JSON
	var decoded execution.ExecutionResult
	err = json.Unmarshal([]byte(output), &decoded)
	require.NoError(t, err)

	// Verify content
	assert.Equal(t, "test-profile", decoded.ProfileName)
	assert.Equal(t, "1.0.0", decoded.ProfileVersion)
	assert.Len(t, decoded.Controls, 3)
	assert.Equal(t, 3, decoded.Summary.TotalControls)
	assert.Equal(t, 4, decoded.Summary.TotalObservations)

	// Verify indentation (pretty-printed)
	assert.Contains(t, output, "  ")
	assert.Contains(t, output, "\n")
}

func TestJSONFormatter_Format_Compact(t *testing.T) {
	t.Parallel()
	result := createTestResult()
	var buf bytes.Buffer

	formatter := NewJSONFormatter(&buf, false)
	err := formatter.Format(result)
	require.NoError(t, err)

	output := buf.String()

	// Verify it's valid JSON
	var decoded execution.ExecutionResult
	err = json.Unmarshal([]byte(output), &decoded)
	require.NoError(t, err)

	// Verify content
	assert.Equal(t, "test-profile", decoded.ProfileName)
	assert.Equal(t, "1.0.0", decoded.ProfileVersion)
	assert.Len(t, decoded.Controls, 3)

	// Verify no indentation (compact)
	lines := strings.Split(output, "\n")
	// Should be mostly one line (plus trailing newline)
	assert.LessOrEqual(t, len(lines), 3)
}

func TestYAMLFormatter_Format(t *testing.T) {
	t.Parallel()
	result := createTestResult()
	var buf bytes.Buffer

	formatter := NewYAMLFormatter(&buf)
	err := formatter.Format(result)
	require.NoError(t, err)

	output := buf.String()

	// Verify it's valid YAML
	var decoded execution.ExecutionResult
	err = yaml.Unmarshal([]byte(output), &decoded)
	require.NoError(t, err)

	// Verify content
	assert.Equal(t, "test-profile", decoded.ProfileName)
	assert.Equal(t, "1.0.0", decoded.ProfileVersion)
	assert.Len(t, decoded.Controls, 3)
	assert.Equal(t, 3, decoded.Summary.TotalControls)
	assert.Equal(t, 4, decoded.Summary.TotalObservations)

	// Verify YAML structure
	assert.Contains(t, output, "profile_name: test-profile")
	assert.Contains(t, output, "profile_version: 1.0.0")
	assert.Contains(t, output, "controls:")
	assert.Contains(t, output, "summary:")
}

func TestAllFormatters_WithSameData(t *testing.T) {
	t.Parallel()
	result := createTestResult()

	// Test that all formatters can handle the same data
	formatters := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "Table",
			test: func(t *testing.T) {
				var buf bytes.Buffer
				formatter := NewTableFormatter(&buf)
				err := formatter.Format(result)
				assert.NoError(t, err)
				assert.NotEmpty(t, buf.String())
			},
		},
		{
			name: "JSON",
			test: func(t *testing.T) {
				var buf bytes.Buffer
				formatter := NewJSONFormatter(&buf, true)
				err := formatter.Format(result)
				assert.NoError(t, err)
				assert.NotEmpty(t, buf.String())
			},
		},
		{
			name: "YAML",
			test: func(t *testing.T) {
				var buf bytes.Buffer
				formatter := NewYAMLFormatter(&buf)
				err := formatter.Format(result)
				assert.NoError(t, err)
				assert.NotEmpty(t, buf.String())
			},
		},
	}

	for _, tc := range formatters {
		t.Run(tc.name, tc.test)
	}
}

func TestTableFormatter_StatusSymbols(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	formatter := NewTableFormatter(&buf)

	tests := []struct {
		status   values.Status
		expected string
	}{
		{values.StatusPass, "✓"},
		{values.StatusFail, "✗"},
		{values.StatusError, "⚠"},
		{"unknown", "?"},
	}

	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			symbol := formatter.getStatusSymbol(tc.status)
			assert.Equal(t, tc.expected, symbol)
		})
	}
}

func TestJSONFormatter_PreservesTypes(t *testing.T) {
	t.Parallel()
	result := createTestResult()
	var buf bytes.Buffer

	formatter := NewJSONFormatter(&buf, true)
	err := formatter.Format(result)
	require.NoError(t, err)

	// Decode and verify types are preserved
	var decoded execution.ExecutionResult
	err = json.Unmarshal(buf.Bytes(), &decoded)
	require.NoError(t, err)

	// Verify durations are preserved
	assert.Greater(t, decoded.Duration, time.Duration(0))
	assert.Greater(t, decoded.Controls[0].Duration, time.Duration(0))
	assert.Greater(t, decoded.Controls[0].Observations[0].Duration, time.Duration(0))

	// Verify status types
	assert.Equal(t, values.StatusPass, decoded.Controls[0].Status)
	assert.Equal(t, values.StatusFail, decoded.Controls[1].Status)
	assert.Equal(t, values.StatusError, decoded.Controls[2].Status)
}

func TestYAMLFormatter_PreservesTypes(t *testing.T) {
	t.Parallel()
	result := createTestResult()
	var buf bytes.Buffer

	formatter := NewYAMLFormatter(&buf)
	err := formatter.Format(result)
	require.NoError(t, err)

	// Decode and verify types are preserved
	var decoded execution.ExecutionResult
	err = yaml.Unmarshal(buf.Bytes(), &decoded)
	require.NoError(t, err)

	// Verify basic fields
	assert.Equal(t, result.ProfileName, decoded.ProfileName)
	assert.Equal(t, result.ProfileVersion, decoded.ProfileVersion)
	assert.Len(t, decoded.Controls, len(result.Controls))

	// Verify status types
	assert.Equal(t, values.StatusPass, decoded.Controls[0].Status)
	assert.Equal(t, values.StatusFail, decoded.Controls[1].Status)
	assert.Equal(t, values.StatusError, decoded.Controls[2].Status)
}
