package engine

import (
	"context"
	"testing"
	"time"

	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/execution"
	"github.com/reglet-dev/reglet/internal/domain/values"
	"github.com/reglet-dev/reglet/internal/infrastructure/build"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSlowExecutor simulates a slow plugin execution.
type mockSlowExecutor struct {
	delay time.Duration
}

func (m *mockSlowExecutor) Execute(ctx context.Context, obs entities.ObservationDefinition) execution.ObservationResult {
	select {
	case <-time.After(m.delay):
		return execution.ObservationResult{Status: values.StatusPass}
	case <-ctx.Done():
		return execution.ObservationResult{
			Status:   values.StatusError,
			RawError: ctx.Err(),
			Error:    &execution.PluginError{Code: "timeout", Message: ctx.Err().Error()},
		}
	}
}

func TestExecute_CompletesBeforeTimeout(t *testing.T) {
	t.Parallel()
	// timeout 50ms, delay 10ms -> Success
	timeout := 50 * time.Millisecond
	delay := 10 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	engine, err := NewEngineWithConfig(context.Background(), build.Get(), DefaultExecutionConfig())
	require.NoError(t, err)
	// Inject mock executor
	engine.executor = &mockSlowExecutor{delay: delay}

	profile := createTestProfile()
	result, err := engine.Execute(ctx, profile)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Controls, 1)
	assert.Equal(t, values.StatusPass, result.Controls[0].Status)
}

func TestExecute_ExceedsTimeout(t *testing.T) {
	t.Parallel()
	// timeout 10ms, delay 50ms -> Error
	timeout := 10 * time.Millisecond
	delay := 50 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	engine, err := NewEngineWithConfig(context.Background(), build.Get(), DefaultExecutionConfig())
	require.NoError(t, err)
	engine.executor = &mockSlowExecutor{delay: delay}

	profile := createTestProfile()
	start := time.Now()
	result, err := engine.Execute(ctx, profile)
	duration := time.Since(start)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "execution timed out")
	assert.Nil(t, result)

	// Should exit roughly at timeout (allow some buffer for scheduler)
	assert.Less(t, duration, 200*time.Millisecond)
}

func TestExecute_ZeroTimeout(t *testing.T) {
	t.Parallel()
	// timeout 0 (infinite), delay 10ms -> Success
	delay := 10 * time.Millisecond

	// Pass background context (no timeout)
	ctx := context.Background()

	engine, err := NewEngineWithConfig(ctx, build.Get(), DefaultExecutionConfig())
	require.NoError(t, err)
	engine.executor = &mockSlowExecutor{delay: delay}

	profile := createTestProfile()
	result, err := engine.Execute(ctx, profile)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, values.StatusPass, result.Controls[0].Status)
}

func createTestProfile() *entities.Profile {
	return &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "test", Version: "1.0"},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "slow-control",
					Name: "Slow Control",
					ObservationDefinitions: []entities.ObservationDefinition{
						{Plugin: "mock", Config: nil},
					},
				},
			},
		},
	}
}
