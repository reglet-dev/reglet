package engine

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/whiskeyjimbo/reglet/internal/config"
	"github.com/whiskeyjimbo/reglet/internal/wasm"
)

func TestNewEngine(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	engine, err := NewEngine(ctx)
	require.NoError(t, err)
	require.NotNil(t, engine)
	require.NotNil(t, engine.runtime)
	require.NotNil(t, engine.executor)

	// Cleanup
	err = engine.Close(ctx)
	assert.NoError(t, err)
}

func TestAggregateControlStatus_AllPass(t *testing.T) {
	t.Parallel()
	observations := []ObservationResult{
		{Status: StatusPass},
		{Status: StatusPass},
		{Status: StatusPass},
	}

	status := aggregateControlStatus(observations)
	assert.Equal(t, StatusPass, status)
}

func TestAggregateControlStatus_OneFail(t *testing.T) {
	t.Parallel()
	observations := []ObservationResult{
		{Status: StatusPass},
		{Status: StatusFail},
		{Status: StatusPass},
	}

	status := aggregateControlStatus(observations)
	assert.Equal(t, StatusFail, status)
}

func TestAggregateControlStatus_OneError(t *testing.T) {
	t.Parallel()
	observations := []ObservationResult{
		{Status: StatusPass},
		{Status: StatusError, Error: &wasm.PluginError{Code: "test", Message: "test error"}},
		{Status: StatusPass},
	}

	status := aggregateControlStatus(observations)
	assert.Equal(t, StatusError, status)
}

func TestAggregateControlStatus_FailTakesPrecedenceOverError(t *testing.T) {
	t.Parallel()
	observations := []ObservationResult{
		{Status: StatusPass},
		{Status: StatusFail},
		{Status: StatusError, Error: &wasm.PluginError{Code: "test", Message: "test error"}},
	}

	// CRITICAL: Fail should take precedence over Error for compliance reporting
	// If we proved non-compliance (fail), that's more important than a technical error
	// Scenario: 9 observations FAIL, 1 errors -> control should be FAIL (not ERROR)
	status := aggregateControlStatus(observations)
	assert.Equal(t, StatusFail, status, "Proven failures must take precedence over errors")
}

func TestAggregateControlStatus_ErrorWithoutFail(t *testing.T) {
	t.Parallel()
	observations := []ObservationResult{
		{Status: StatusPass},
		{Status: StatusPass},
		{Status: StatusError, Error: &wasm.PluginError{Code: "test", Message: "test error"}},
	}

	// Errors should still be reported when there are no failures
	status := aggregateControlStatus(observations)
	assert.Equal(t, StatusError, status, "Errors should be reported when there are no proven failures")
}

func TestAggregateControlStatus_NoObservations(t *testing.T) {
	t.Parallel()
	observations := []ObservationResult{}

	status := aggregateControlStatus(observations)
	assert.Equal(t, StatusError, status)
}

func TestGenerateControlMessage_SinglePass(t *testing.T) {
	t.Parallel()
	observations := []ObservationResult{
		{Status: StatusPass},
	}

	msg := generateControlMessage(StatusPass, observations)
	assert.Equal(t, "Check passed", msg)
}

func TestGenerateControlMessage_MultiplePass(t *testing.T) {
	t.Parallel()
	observations := []ObservationResult{
		{Status: StatusPass},
		{Status: StatusPass},
		{Status: StatusPass},
	}

	msg := generateControlMessage(StatusPass, observations)
	assert.Equal(t, "All 3 checks passed", msg)
}

func TestGenerateControlMessage_SingleFail(t *testing.T) {
	t.Parallel()
	observations := []ObservationResult{
		{Status: StatusPass},
		{Status: StatusFail},
	}

	msg := generateControlMessage(StatusFail, observations)
	assert.Equal(t, "1 check failed", msg)
}

func TestGenerateControlMessage_MultipleFail(t *testing.T) {
	t.Parallel()
	observations := []ObservationResult{
		{Status: StatusFail},
		{Status: StatusFail},
		{Status: StatusPass},
	}

	msg := generateControlMessage(StatusFail, observations)
	assert.Equal(t, "2 checks failed", msg)
}

func TestGenerateControlMessage_SingleError(t *testing.T) {
	t.Parallel()
	observations := []ObservationResult{
		{
			Status: StatusError,
			Error:  &wasm.PluginError{Code: "test", Message: "something went wrong"},
		},
	}

	msg := generateControlMessage(StatusError, observations)
	assert.Equal(t, "something went wrong", msg)
}

func TestGenerateControlMessage_SingleErrorNoMessage(t *testing.T) {
	t.Parallel()
	observations := []ObservationResult{
		{
			Status: StatusError,
			Error:  nil, // No error object
		},
	}

	msg := generateControlMessage(StatusError, observations)
	assert.Equal(t, "Check encountered an error", msg)
}

func TestGenerateControlMessage_MultipleErrors(t *testing.T) {
	t.Parallel()
	observations := []ObservationResult{
		{Status: StatusError, Error: &wasm.PluginError{Code: "test", Message: "error 1"}},
		{Status: StatusError, Error: &wasm.PluginError{Code: "test", Message: "error 2"}},
		{Status: StatusPass},
	}

	msg := generateControlMessage(StatusError, observations)
	assert.Equal(t, "2 checks encountered errors", msg)
}

func TestExecuteControl_SingleObservation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine, err := NewEngine(ctx)
	require.NoError(t, err)
	defer engine.Close(ctx)

	ctrl := config.Control{
		ID:          "test-control",
		Name:        "Test Control",
		Description: "A test control",
		Severity:    "medium",
		Tags:        []string{"test"},
		Observations: []config.Observation{
			{
				Plugin: "file",
				Config: map[string]interface{}{
					"path":   "/tmp/test.txt",
					"mode":   "exists",
					"status": true,
				},
			},
		},
	}

	// Create empty execution result for dependency checking
	execResult := NewExecutionResult("test", "1.0.0")
	result := engine.executeControl(ctx, ctrl, execResult)

	assert.Equal(t, "test-control", result.ID)
	assert.Equal(t, "Test Control", result.Name)
	assert.Equal(t, "A test control", result.Description)
	assert.Equal(t, "medium", result.Severity)
	assert.Equal(t, []string{"test"}, result.Tags)
	assert.Len(t, result.Observations, 1)
	assert.Greater(t, result.Duration, time.Duration(0))
	assert.NotEmpty(t, result.Message)
}

func TestExecuteControl_MultipleObservations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine, err := NewEngine(ctx)
	require.NoError(t, err)
	defer engine.Close(ctx)

	ctrl := config.Control{
		ID:   "multi-test",
		Name: "Multi Observation Test",
		Observations: []config.Observation{
			{
				Plugin: "file",
				Config: map[string]interface{}{
					"path": "/tmp/test1.txt",
					"mode": "exists",
				},
			},
			{
				Plugin: "file",
				Config: map[string]interface{}{
					"path": "/tmp/test2.txt",
					"mode": "exists",
				},
			},
		},
	}

	// Create empty execution result for dependency checking
	execResult := NewExecutionResult("test", "1.0.0")
	result := engine.executeControl(ctx, ctrl, execResult)

	assert.Equal(t, "multi-test", result.ID)
	assert.Len(t, result.Observations, 2)
	assert.Greater(t, result.Duration, time.Duration(0))
}

func TestExecute_SingleControl(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine, err := NewEngine(ctx)
	require.NoError(t, err)
	defer engine.Close(ctx)

	profile := &config.Profile{
		Metadata: config.ProfileMetadata{
			Name:    "test-profile",
			Version: "1.0.0",
		},
		Controls: config.ControlsSection{
			Items: []config.Control{
				{
					ID:   "control-1",
					Name: "Control 1",
					Observations: []config.Observation{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/tmp/test.txt",
								"mode": "exists",
							},
						},
					},
				},
			},
		},
	}

	result, err := engine.Execute(ctx, profile)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "test-profile", result.ProfileName)
	assert.Equal(t, "1.0.0", result.ProfileVersion)
	assert.NotZero(t, result.StartTime)
	assert.NotZero(t, result.EndTime)
	assert.Greater(t, result.Duration, time.Duration(0))
	assert.Len(t, result.Controls, 1)
	assert.Equal(t, 1, result.Summary.TotalControls)
	assert.Equal(t, 1, result.Summary.TotalObservations)
}

func TestExecute_MultipleControls(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine, err := NewEngine(ctx)
	require.NoError(t, err)
	defer engine.Close(ctx)

	profile := &config.Profile{
		Metadata: config.ProfileMetadata{
			Name:    "multi-control-profile",
			Version: "2.0.0",
		},
		Controls: config.ControlsSection{
			Items: []config.Control{
				{
					ID:   "control-1",
					Name: "Control 1",
					Observations: []config.Observation{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/tmp/test1.txt",
								"mode": "exists",
							},
						},
					},
				},
				{
					ID:   "control-2",
					Name: "Control 2",
					Observations: []config.Observation{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/tmp/test2.txt",
								"mode": "exists",
							},
						},
					},
				},
			},
		},
	}

	result, err := engine.Execute(ctx, profile)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "multi-control-profile", result.ProfileName)
	assert.Len(t, result.Controls, 2)
	assert.Equal(t, 2, result.Summary.TotalControls)
	assert.Equal(t, 2, result.Summary.TotalObservations)
}

func TestExecute_SummaryStatistics(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine, err := NewEngine(ctx)
	require.NoError(t, err)
	defer engine.Close(ctx)

	profile := &config.Profile{
		Metadata: config.ProfileMetadata{
			Name:    "summary-test",
			Version: "1.0.0",
		},
		Controls: config.ControlsSection{
			Items: []config.Control{
				{
					ID:   "control-1",
					Name: "Control 1",
					Observations: []config.Observation{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/tmp/test.txt",
								"mode": "exists",
							},
						},
					},
				},
			},
		},
	}

	result, err := engine.Execute(ctx, profile)
	require.NoError(t, err)

	// Verify summary is calculated
	assert.Equal(t, 1, result.Summary.TotalControls)
	assert.Equal(t, 1, result.Summary.TotalObservations)

	// Should have exactly one of: pass, fail, or error
	totalStatusCounts := result.Summary.PassedControls +
		result.Summary.FailedControls +
		result.Summary.ErrorControls
	assert.Equal(t, 1, totalStatusCounts)
}

func TestExecute_TimingInfo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine, err := NewEngine(ctx)
	require.NoError(t, err)
	defer engine.Close(ctx)

	profile := &config.Profile{
		Metadata: config.ProfileMetadata{
			Name:    "timing-test",
			Version: "1.0.0",
		},
		Controls: config.ControlsSection{
			Items: []config.Control{
				{
					ID:   "control-1",
					Name: "Control 1",
					Observations: []config.Observation{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/tmp/test.txt",
								"mode": "exists",
							},
						},
					},
				},
			},
		},
	}

	result, err := engine.Execute(ctx, profile)
	require.NoError(t, err)

	// Verify timing information is present
	assert.NotZero(t, result.StartTime)
	assert.NotZero(t, result.EndTime)
	assert.Greater(t, result.Duration, time.Duration(0))
	assert.True(t, result.EndTime.After(result.StartTime))
	assert.Greater(t, result.Controls[0].Duration, time.Duration(0))
	assert.Greater(t, result.Controls[0].Observations[0].Duration, time.Duration(0))
}

func TestExecute_InvalidPlugin(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine, err := NewEngine(ctx)
	require.NoError(t, err)
	defer engine.Close(ctx)

	profile := &config.Profile{
		Metadata: config.ProfileMetadata{
			Name:    "invalid-plugin-test",
			Version: "1.0.0",
		},
		Controls: config.ControlsSection{
			Items: []config.Control{
				{
					ID:   "control-1",
					Name: "Control 1",
					Observations: []config.Observation{
						{
							Plugin: "nonexistent-plugin",
							Config: map[string]interface{}{
								"test": "value",
							},
						},
					},
				},
			},
		},
	}

	result, err := engine.Execute(ctx, profile)
	require.NoError(t, err) // Execute should not return error, but result should show error

	assert.Len(t, result.Controls, 1)
	assert.Equal(t, StatusError, result.Controls[0].Status)
	assert.Len(t, result.Controls[0].Observations, 1)
	assert.Equal(t, StatusError, result.Controls[0].Observations[0].Status)
	assert.NotNil(t, result.Controls[0].Observations[0].Error)
	assert.Contains(t, result.Controls[0].Observations[0].Error.Message, "failed to read plugin")
}
