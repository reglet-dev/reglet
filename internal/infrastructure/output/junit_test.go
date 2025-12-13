package output

import (
	"bytes"
	"encoding/xml"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/whiskeyjimbo/reglet/internal/domain/execution"
	"github.com/whiskeyjimbo/reglet/internal/domain/values"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/wasm"
)

func TestJUnitFormatter_Format(t *testing.T) {
	result := execution.NewExecutionResult("test-profile", "1.0.0")

	// Add passing control
	passingCtrl := execution.ControlResult{
		ID:       "ctrl-1",
		Name:     "Passing Control",
		Status:   values.StatusPass,
		Duration: 100 * time.Millisecond,
		Observations: []execution.ObservationResult{
			{Plugin: "test", Status: values.StatusPass},
		},
	}

	// Add failing control
	failingCtrl := execution.ControlResult{
		ID:       "ctrl-2",
		Name:     "Failing Control",
		Status:   values.StatusFail,
		Message:  "Control failed",
		Duration: 50 * time.Millisecond,
		Observations: []execution.ObservationResult{
			{
				Plugin: "test",
				Status: values.StatusFail,
				Evidence: &wasm.Evidence{
					Data: map[string]interface{}{"key": "value"},
				},
			},
		},
	}

	// Add error control
	errorCtrl := execution.ControlResult{
		ID:       "ctrl-3",
		Name:     "Error Control",
		Status:   values.StatusError,
		Message:  "Control error",
		Duration: 10 * time.Millisecond,
		Observations: []execution.ObservationResult{
			{
				Plugin: "test",
				Status: values.StatusError,
				Error:  &wasm.PluginError{Message: "Internal error"},
			},
		},
	}

	// Add skipped control
	skippedCtrl := execution.ControlResult{
		ID:         "ctrl-4",
		Name:       "Skipped Control",
		Status:     values.StatusSkipped,
		SkipReason: "Not applicable",
		Duration:   0,
	}

	result.AddControlResult(passingCtrl)
	result.AddControlResult(failingCtrl)
	result.AddControlResult(errorCtrl)
	result.AddControlResult(skippedCtrl)
	result.Finalize()

	var buf bytes.Buffer
	formatter := NewJUnitFormatter(&buf)
	err := formatter.Format(result)
	require.NoError(t, err)

	// Verify XML structure
	var suites JUnitTestSuites
	err = xml.Unmarshal(buf.Bytes(), &suites)
	require.NoError(t, err)

	assert.Equal(t, "Reglet Execution", suites.Name)
	assert.Equal(t, 4, suites.Tests)
	assert.Equal(t, 1, suites.Failures)
	assert.Equal(t, 1, suites.Errors)

	require.Len(t, suites.TestSuites, 1)
	suite := suites.TestSuites[0]
	assert.Equal(t, "test-profile", suite.Name)
	assert.Equal(t, 4, suite.Tests)
	assert.Equal(t, 1, suite.Failures)
	assert.Equal(t, 1, suite.Errors)
	assert.Equal(t, 1, suite.Skipped)

	require.Len(t, suite.TestCases, 4)

	// Check passing case
	assert.Equal(t, "ctrl-1", suite.TestCases[0].Name)
	assert.Nil(t, suite.TestCases[0].Failure)
	assert.Nil(t, suite.TestCases[0].Error)
	assert.Nil(t, suite.TestCases[0].Skipped)

	// Check failing case
	assert.Equal(t, "ctrl-2", suite.TestCases[1].Name)
	assert.NotNil(t, suite.TestCases[1].Failure)
	assert.Equal(t, "Control failed", suite.TestCases[1].Failure.Message)
	assert.Contains(t, suite.TestCases[1].Failure.Content, "Evidence: map[key:value]")

	// Check error case
	assert.Equal(t, "ctrl-3", suite.TestCases[2].Name)
	assert.NotNil(t, suite.TestCases[2].Error)
	assert.Equal(t, "Control error", suite.TestCases[2].Error.Message)
	assert.Contains(t, suite.TestCases[2].Error.Content, "Error: Internal error")

	// Check skipped case
	assert.Equal(t, "ctrl-4", suite.TestCases[3].Name)
	assert.NotNil(t, suite.TestCases[3].Skipped)
	assert.Equal(t, "Not applicable", suite.TestCases[3].Skipped.Message)
}
