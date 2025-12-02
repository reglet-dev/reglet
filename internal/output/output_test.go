package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/whiskeyjimbo/reglet/internal/engine"
	"github.com/whiskeyjimbo/reglet/internal/wasm"
	"gopkg.in/yaml.v3"
)

// createTestResult creates a sample execution result for testing.
func createTestResult() *engine.ExecutionResult {
	result := engine.NewExecutionResult("test-profile", "1.0.0")

	// Add a passing control
	passingControl := engine.ControlResult{
		ID:          "ctrl-1",
		Name:        "Test Control 1",
		Description: "A test control that passes",
		Severity:    "high",
		Tags:        []string{"security", "test"},
		Status:      engine.StatusPass,
		Message:     "All 2 checks passed",
		Duration:    100 * time.Millisecond,
		Observations: []engine.ObservationResult{
			{
				Plugin: "file",
				Config: map[string]interface{}{
					"path": "/etc/test",
					"mode": "exists",
				},
				Status: engine.StatusPass,
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
				Status: engine.StatusPass,
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
	failingControl := engine.ControlResult{
		ID:          "ctrl-2",
		Name:        "Test Control 2",
		Description: "A test control that fails",
		Severity:    "medium",
		Status:      engine.StatusFail,
		Message:     "1 check failed",
		Duration:    50 * time.Millisecond,
		Observations: []engine.ObservationResult{
			{
				Plugin: "file",
				Config: map[string]interface{}{
					"path": "/etc/missing",
					"mode": "exists",
				},
				Status: engine.StatusFail,
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
	errorControl := engine.ControlResult{
		ID:       "ctrl-3",
		Name:     "Test Control 3",
		Severity: "critical",
		Status:   engine.StatusError,
		Message:  "Plugin load failed",
		Duration: 10 * time.Millisecond,
		Observations: []engine.ObservationResult{
			{
				Plugin: "nonexistent",
				Config: map[string]interface{}{
					"test": "value",
				},
				Status: engine.StatusError,
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
	t.Parallel()
	result := createTestResult()
	var buf bytes.Buffer

	formatter := NewTableFormatter(&buf)
	err := formatter.Format(result)
	require.NoError(t, err)

	output := buf.String()

	// Check header
	assert.Contains(t, output, "Profile: test-profile (v1.0.0)")
	assert.Contains(t, output, "Executed:")
	assert.Contains(t, output, "Duration:")

	// Check controls section
	assert.Contains(t, output, "Controls:")
	assert.Contains(t, output, "ctrl-1: Test Control 1")
	assert.Contains(t, output, "ctrl-2: Test Control 2")
	assert.Contains(t, output, "ctrl-3: Test Control 3")

	// Check status symbols
	assert.Contains(t, output, "✓") // Pass
	assert.Contains(t, output, "✗") // Fail
	assert.Contains(t, output, "⚠") // Error

	// Check summary section
	assert.Contains(t, output, "Summary:")
	assert.Contains(t, output, "Controls:     3 total")
	assert.Contains(t, output, "Observations: 4 total")

	// Check specific control details
	assert.Contains(t, output, "A test control that passes")
	assert.Contains(t, output, "Severity: high")
	assert.Contains(t, output, "Tags: security, test")
	assert.Contains(t, output, "Plugin load failed")
}

func TestTableFormatter_EmptyResult(t *testing.T) {
	t.Parallel()
	result := engine.NewExecutionResult("empty-profile", "1.0.0")
	result.Finalize()

	var buf bytes.Buffer
	formatter := NewTableFormatter(&buf)
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
	var decoded engine.ExecutionResult
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
	var decoded engine.ExecutionResult
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
	var decoded engine.ExecutionResult
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
		status   engine.Status
		expected string
	}{
		{engine.StatusPass, "✓"},
		{engine.StatusFail, "✗"},
		{engine.StatusError, "⚠"},
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
	var decoded engine.ExecutionResult
	err = json.Unmarshal(buf.Bytes(), &decoded)
	require.NoError(t, err)

	// Verify durations are preserved
	assert.Greater(t, decoded.Duration, time.Duration(0))
	assert.Greater(t, decoded.Controls[0].Duration, time.Duration(0))
	assert.Greater(t, decoded.Controls[0].Observations[0].Duration, time.Duration(0))

	// Verify status types
	assert.Equal(t, engine.StatusPass, decoded.Controls[0].Status)
	assert.Equal(t, engine.StatusFail, decoded.Controls[1].Status)
	assert.Equal(t, engine.StatusError, decoded.Controls[2].Status)
}

func TestYAMLFormatter_PreservesTypes(t *testing.T) {
	t.Parallel()
	result := createTestResult()
	var buf bytes.Buffer

	formatter := NewYAMLFormatter(&buf)
	err := formatter.Format(result)
	require.NoError(t, err)

	// Decode and verify types are preserved
	var decoded engine.ExecutionResult
	err = yaml.Unmarshal(buf.Bytes(), &decoded)
	require.NoError(t, err)

	// Verify basic fields
	assert.Equal(t, result.ProfileName, decoded.ProfileName)
	assert.Equal(t, result.ProfileVersion, decoded.ProfileVersion)
	assert.Len(t, decoded.Controls, len(result.Controls))

	// Verify status types
	assert.Equal(t, engine.StatusPass, decoded.Controls[0].Status)
	assert.Equal(t, engine.StatusFail, decoded.Controls[1].Status)
	assert.Equal(t, engine.StatusError, decoded.Controls[2].Status)
}
