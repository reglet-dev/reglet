package output

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/owenrumney/go-sarif/v3/pkg/report/v210/sarif"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/whiskeyjimbo/reglet/internal/domain/execution"
	"github.com/whiskeyjimbo/reglet/internal/domain/values"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/wasm"
)

func TestSARIFFormatter_Format(t *testing.T) {
	t.Parallel()
	result := createSARIFTestResult()
	var buf bytes.Buffer

	formatter := NewSARIFFormatter(&buf, "test-profile.yaml")
	err := formatter.Format(result)
	require.NoError(t, err)

	output := buf.String()

	// Verify it's valid JSON
	var raw map[string]interface{}
	err = json.Unmarshal([]byte(output), &raw)
	require.NoError(t, err)

	// Verify SARIF structure
	assert.Equal(t, "2.1.0", raw["version"])
	assert.Contains(t, raw, "$schema")
	assert.Contains(t, raw, "runs")

	runs := raw["runs"].([]interface{})
	require.Len(t, runs, 1)

	run := runs[0].(map[string]interface{})
	assert.Contains(t, run, "tool")
	assert.Contains(t, run, "results")
	assert.Contains(t, run, "invocations")
}

func TestSARIFFormatter_ValidatesAgainstSchema(t *testing.T) {
	t.Parallel()
	result := createSARIFTestResult()
	var buf bytes.Buffer

	formatter := NewSARIFFormatter(&buf, "test-profile.yaml")
	err := formatter.Format(result)
	require.NoError(t, err)

	// Parse back into go-sarif for validation
	report, err := sarif.FromBytes(buf.Bytes())
	require.NoError(t, err)

	// Validate against SARIF schema
	err = report.Validate()
	require.NoError(t, err)
}

func TestSARIFFormatter_ToolMetadata(t *testing.T) {
	t.Parallel()
	result := createSARIFTestResult()
	result.RegletVersion = "1.2.3"
	var buf bytes.Buffer

	formatter := NewSARIFFormatter(&buf, "")
	err := formatter.Format(result)
	require.NoError(t, err)

	report, err := sarif.FromBytes(buf.Bytes())
	require.NoError(t, err)
	require.Len(t, report.Runs, 1)

	tool := report.Runs[0].Tool
	assert.Equal(t, "Reglet", *tool.Driver.Name)
	assert.Equal(t, "1.2.3", *tool.Driver.Version)
	assert.Equal(t, "https://reglet.dev", *tool.Driver.InformationURI)
}

func TestSARIFFormatter_StatusLevelMapping(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		status    values.Status
		severity  string
		wantLevel string
		wantKind  string
	}{
		{"pass", values.StatusPass, "high", "note", "pass"},
		{"fail-critical", values.StatusFail, "critical", "error", "fail"},
		{"fail-high", values.StatusFail, "high", "error", "fail"},
		{"fail-medium", values.StatusFail, "medium", "warning", "fail"},
		{"fail-low", values.StatusFail, "low", "warning", "fail"},
		{"fail-unknown", values.StatusFail, "", "warning", "fail"},
		{"error", values.StatusError, "medium", "error", "fail"},
		{"skipped", values.StatusSkipped, "low", "none", "notApplicable"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := execution.NewExecutionResult("test", "1.0.0")
			result.AddControlResult(execution.ControlResult{
				ID:       "test-ctrl",
				Name:     "Test",
				Status:   tc.status,
				Severity: tc.severity,
				Message:  "test message",
			})
			result.Finalize()

			var buf bytes.Buffer
			formatter := NewSARIFFormatter(&buf, "")
			err := formatter.Format(result)
			require.NoError(t, err)

			report, err := sarif.FromBytes(buf.Bytes())
			require.NoError(t, err)
			require.Len(t, report.Runs[0].Results, 1)

			res := report.Runs[0].Results[0]
			assert.Equal(t, tc.wantLevel, res.Level, "level mismatch")
			assert.Equal(t, tc.wantKind, res.Kind, "kind mismatch")
		})
	}
}

func TestSARIFMapper_ExtractLocation_Path(t *testing.T) {
	t.Parallel()
	result := execution.NewExecutionResult("test", "1.0.0")
	ctrl := execution.ControlResult{ID: "loc-1", Status: values.StatusPass}
	ctrl.Observations = []execution.ObservationResult{
		{
			Evidence: &wasm.Evidence{
				Data: map[string]interface{}{
					"path":   "/etc/hosts",
					"line":   10,
					"column": 5,
				},
			},
		},
	}
	result.AddControlResult(ctrl)
	result.Finalize()

	report := formatToReport(t, result)
	res := report.Runs[0].Results[0]
	require.Len(t, res.Locations, 1)
	loc := res.Locations[0]
	assert.Equal(t, "file:///etc/hosts", *loc.PhysicalLocation.ArtifactLocation.URI)
	assert.Equal(t, 10, *loc.PhysicalLocation.Region.StartLine)
	assert.Equal(t, 5, *loc.PhysicalLocation.Region.StartColumn)
}

func TestSARIFMapper_ExtractLocation_CommandPath(t *testing.T) {
	t.Parallel()
	result := execution.NewExecutionResult("test", "1.0.0")
	ctrl := execution.ControlResult{ID: "loc-2", Status: values.StatusPass}
	ctrl.Observations = []execution.ObservationResult{
		{
			Evidence: &wasm.Evidence{
				Data: map[string]interface{}{
					"command_path": "/usr/bin/ls",
				},
			},
		},
	}
	result.AddControlResult(ctrl)
	result.Finalize()

	report := formatToReport(t, result)
	res := report.Runs[0].Results[0]
	loc := res.Locations[0]
	assert.Equal(t, "file:///usr/bin/ls", *loc.PhysicalLocation.ArtifactLocation.URI)
}

func TestSARIFMapper_ExtractLocation_ShellCommand_Valid(t *testing.T) {
	t.Parallel()
	result := execution.NewExecutionResult("test", "1.0.0")
	ctrl := execution.ControlResult{ID: "loc-3", Status: values.StatusPass}
	ctrl.Observations = []execution.ObservationResult{
		{
			Evidence: &wasm.Evidence{
				Data: map[string]interface{}{
					"shell_command": "/usr/local/bin/myscript.sh",
				},
			},
		},
	}
	result.AddControlResult(ctrl)
	result.Finalize()

	report := formatToReport(t, result)
	res := report.Runs[0].Results[0]
	loc := res.Locations[0]
	assert.Equal(t, "file:///usr/local/bin/myscript.sh", *loc.PhysicalLocation.ArtifactLocation.URI)
}

func TestSARIFMapper_ExtractLocation_ShellCommand_Invalid(t *testing.T) {
	t.Parallel()
	result := execution.NewExecutionResult("test", "1.0.0")
	ctrl := execution.ControlResult{ID: "loc-4", Status: values.StatusPass}
	ctrl.Observations = []execution.ObservationResult{
		{
			Evidence: &wasm.Evidence{
				Data: map[string]interface{}{
					"shell_command": "ls -la /tmp", // Inline command, not a file path
				},
			},
		},
	}
	result.AddControlResult(ctrl)
	result.Finalize()

	report := formatToReport(t, result)
	res := report.Runs[0].Results[0]
	assert.Empty(t, res.Locations, "Should not extract location for inline command")
}

func TestSARIFMapper_ArtifactRegistration_Deduplication(t *testing.T) {
	t.Parallel()
	result := execution.NewExecutionResult("test", "1.0.0")
	// Two controls pointing to same file
	ctrl1 := execution.ControlResult{ID: "dup-1", Status: values.StatusPass}
	ctrl1.Observations = []execution.ObservationResult{
		{Evidence: &wasm.Evidence{Data: map[string]interface{}{"path": "/same/file"}}},
	}
	ctrl2 := execution.ControlResult{ID: "dup-2", Status: values.StatusPass}
	ctrl2.Observations = []execution.ObservationResult{
		{Evidence: &wasm.Evidence{Data: map[string]interface{}{"path": "/same/file"}}},
	}
	result.AddControlResult(ctrl1)
	result.AddControlResult(ctrl2)
	result.Finalize()

	report := formatToReport(t, result)
	run := report.Runs[0]
	assert.Len(t, run.Artifacts, 1, "Artifacts should be deduplicated")
	assert.Equal(t, "file:///same/file", *run.Artifacts[0].Location.URI)
}

func TestSARIFMapper_ArtifactProperties(t *testing.T) {
	t.Parallel()
	result := execution.NewExecutionResult("test", "1.0.0")
	ctrl := execution.ControlResult{ID: "props-1", Status: values.StatusPass}
	ctrl.Observations = []execution.ObservationResult{
		{
			Evidence: &wasm.Evidence{
				Data: map[string]interface{}{
					"path":     "/test/file",
					"size":     1024,
					"exists":   true,
					"readable": false,
				},
			},
		},
	}
	result.AddControlResult(ctrl)
	result.Finalize()

	report := formatToReport(t, result)
	artifact := report.Runs[0].Artifacts[0]
	assert.Equal(t, 1024, artifact.Length)

	props := artifact.Properties.Properties
	assert.Equal(t, true, props["exists"])
	assert.Equal(t, false, props["readable"])
}

func TestSARIFMapper_LocationNormalization_Relative(t *testing.T) {
	// This test relies on CWD, so we need to mock or be careful.
	// sarifMapper uses os.Getwd(). We can't easily mock that without refactoring.
	// But we can check if it normalizes to relative path if we pass absolute path in CWD.
	t.Parallel()

	cwd, err := filepath.Abs(".")
	require.NoError(t, err)
	targetFile := filepath.Join(cwd, "test-file.txt")

	result := execution.NewExecutionResult("test", "1.0.0")
	ctrl := execution.ControlResult{ID: "norm-1", Status: values.StatusPass}
	ctrl.Observations = []execution.ObservationResult{
		{Evidence: &wasm.Evidence{Data: map[string]interface{}{"path": targetFile}}},
	}
	result.AddControlResult(ctrl)
	result.Finalize()

	report := formatToReport(t, result)
	loc := report.Runs[0].Results[0].Locations[0]

	// Expect relative path (no file:// prefix, just the file name or relative path)
	// Actually sarifMapper implementation does: "file://" + filepath.ToSlash(abs) if not relative
	// If relative, it does filepath.ToSlash(rel).
	// Since we are in CWD, it should be relative "test-file.txt".
	// NOTE: sarifMapper logic: if Rel returns no error and !strings.HasPrefix(rel, ".."), use rel.
	assert.Equal(t, "test-file.txt", *loc.PhysicalLocation.ArtifactLocation.URI)
}

func TestSARIFMapper_EmptyResults(t *testing.T) {
	t.Parallel()
	result := execution.NewExecutionResult("test", "1.0.0")
	result.Finalize()

	report := formatToReport(t, result)
	assert.Empty(t, report.Runs[0].Results)
	assert.Empty(t, report.Runs[0].Artifacts)
}

func TestSARIFMapper_PropertiesPreservation(t *testing.T) {
	t.Parallel()
	result := execution.NewExecutionResult("test", "1.0.0")
	ctrl := execution.ControlResult{
		ID:         "prop-pres-1",
		Status:     values.StatusPass,
		Severity:   "critical",
		Tags:       []string{"tag1", "tag2"},
		SkipReason: "not skipped",
	}
	result.AddControlResult(ctrl)
	result.Finalize()

	report := formatToReport(t, result)
	res := report.Runs[0].Results[0]

	props := res.Properties.Properties
	assert.Equal(t, "critical", props["severity"])
	assert.Equal(t, "not skipped", props["skipReason"])

	// Tags are in res.Properties.Tags (slice)
	assert.Contains(t, res.Properties.Tags, "tag1")
	assert.Contains(t, res.Properties.Tags, "tag2")
}

// Helper to format result to Report
func formatToReport(t *testing.T, result *execution.ExecutionResult) *sarif.Report {
	var buf bytes.Buffer
	formatter := NewSARIFFormatter(&buf, "")
	err := formatter.Format(result)
	require.NoError(t, err)

	report, err := sarif.FromBytes(buf.Bytes())
	require.NoError(t, err)
	return report
}

// Helper to create a basic execution result for testing
func createSARIFTestResult() *execution.ExecutionResult {
	res := execution.NewExecutionResult("test-profile", "1.0.0")
	res.StartTime = time.Now().Add(-1 * time.Second)
	res.EndTime = time.Now()

	// Add a passing control
	res.AddControlResult(execution.ControlResult{
		ID:          "CTRL-01",
		Name:        "Pass Control",
		Description: "A passing control",
		Status:      values.StatusPass,
		Severity:    "high",
		Message:     "Control passed",
	})

	res.Finalize()
	return res
}
