package output

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/whiskeyjimbo/reglet/internal/domain"
	"github.com/whiskeyjimbo/reglet/internal/engine"
	"github.com/whiskeyjimbo/reglet/internal/wasm"
)

func TestJUnitFormatter_Format(t *testing.T) {
	// Setup test data
	result := engine.NewExecutionResult("test-profile", "1.0.0")
	result.Duration = 1500 * time.Millisecond

	// Pass Control
	result.AddControlResult(engine.ControlResult{
		ID:       "control-1",
		Name:     "Passed Control",
		Status:   domain.StatusPass,
		Duration: 100 * time.Millisecond,
		Observations: []engine.ObservationResult{
			{Plugin: "test", Status: domain.StatusPass},
		},
	})

	// Fail Control
	result.AddControlResult(engine.ControlResult{
		ID:       "control-2",
		Name:     "Failed Control",
		Status:   domain.StatusFail,
		Message:  "Something failed",
		Duration: 200 * time.Millisecond,
		Observations: []engine.ObservationResult{
			{
				Plugin: "test",
				Status: domain.StatusFail,
				Evidence: &wasm.Evidence{
					Data: map[string]interface{}{"key": "value"},
				},
			},
		},
	})

	// Error Control
	result.AddControlResult(engine.ControlResult{
		ID:       "control-3",
		Name:     "Error Control",
		Status:   domain.StatusError,
		Message:  "Something errored",
		Duration: 300 * time.Millisecond,
		Observations: []engine.ObservationResult{
			{
				Plugin: "test",
				Status: domain.StatusError,
				Error:  &wasm.PluginError{Message: "runtime error"},
			},
		},
	})

	// Skipped Control
	result.AddControlResult(engine.ControlResult{
		ID:         "control-4",
		Name:       "Skipped Control",
		Status:     domain.StatusSkipped,
		SkipReason: "dependency failed",
		Duration:   0,
	})

	result.Finalize()

	// Execute Format
	var buf bytes.Buffer
	formatter := NewJUnitFormatter(&buf)
	err := formatter.Format(result)

	// Assertions
	assert.NoError(t, err)
	output := buf.String()

	// Verify XML structure and content
	assert.Contains(t, output, `<?xml version="1.0" encoding="UTF-8"?>`)
	assert.Contains(t, output, `<testsuites name="Reglet Execution" tests="4" failures="1" errors="1"`)
	assert.Contains(t, output, `<testsuite name="test-profile" tests="4" failures="1" errors="1" skipped="1"`)
	
	// Check Control-1 (Pass)
	assert.Contains(t, output, `<testcase name="control-1" classname="Passed Control"`)
	
	// Check Control-2 (Fail)
	assert.Contains(t, output, `<testcase name="control-2" classname="Failed Control"`)
	assert.Contains(t, output, `<failure message="Something failed"`)
	assert.Contains(t, output, `Observation (test): fail`) // Check content of failure
	assert.Contains(t, output, `Evidence: map[key:value]`)

	// Check Control-3 (Error)
	assert.Contains(t, output, `<testcase name="control-3" classname="Error Control"`)
	assert.Contains(t, output, `<error message="Something errored"`)
	assert.Contains(t, output, `Error: runtime error`)

	// Check Control-4 (Skipped)
	assert.Contains(t, output, `<testcase name="control-4" classname="Skipped Control"`)
	assert.Contains(t, output, `<skipped message="dependency failed"`)
}