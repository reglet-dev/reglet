package engine

import (
	"context"
	"testing"
	"time"

	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/execution"
	"github.com/reglet-dev/reglet/internal/domain/values"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockExecutor is a mock implementation of ObservationExecutable
type MockExecutor struct {
	mock.Mock
}

func (m *MockExecutor) Execute(ctx context.Context, obs entities.ObservationDefinition) execution.ObservationResult {
	args := m.Called(ctx, obs)
	return args.Get(0).(execution.ObservationResult)
}

func TestExecuteControl_RetrySuccess(t *testing.T) {
	mockExec := new(MockExecutor)
	// Initialize Engine with minimal fields required for executeControl
	engine := &Engine{
		executor: mockExec,
		config: ExecutionConfig{
			Parallel: false,
		},
	}

	ctrl := entities.Control{
		ID:           "test-control",
		Name:         "Test Control",
		Retries:      3,
		RetryDelay:   time.Millisecond,
		RetryBackoff: entities.BackoffNone,
		ObservationDefinitions: []entities.ObservationDefinition{
			{Plugin: "test-plugin"},
		},
	}

	// Define transient error (reusing mockTimeoutError from backoff_test.go if available, or define new)
	// Since we are in the same package 'engine', we can share types if they are in _test.go files?
	// Yes, usually. But to be safe, I'll redefine or use a local one.
	transientErr := &mockTimeoutError{}

	// Result 1: Transient Error
	failResult := execution.ObservationResult{
		Status:   values.StatusError,
		RawError: transientErr,
	}

	// Result 2: Success
	passResult := execution.ObservationResult{
		Status: values.StatusPass,
	}

	// Expect 2 calls: Fail -> Success
	mockExec.On("Execute", mock.Anything, mock.Anything).Return(failResult).Once()
	mockExec.On("Execute", mock.Anything, mock.Anything).Return(passResult).Once()

	// Run
	result := engine.executeControl(context.Background(), ctrl, 0, nil, nil)

	// Assert
	assert.Equal(t, values.StatusPass, result.Status)
	mockExec.AssertNumberOfCalls(t, "Execute", 2)
}

func TestExecuteControl_RetryExhausted(t *testing.T) {
	mockExec := new(MockExecutor)
	engine := &Engine{
		executor: mockExec,
		config: ExecutionConfig{
			Parallel: false,
		},
	}

	ctrl := entities.Control{
		ID:           "test-control-fail",
		Name:         "Fail Control",
		Retries:      2, // 2 retries = 3 attempts total
		RetryDelay:   time.Millisecond,
		RetryBackoff: entities.BackoffNone,
		ObservationDefinitions: []entities.ObservationDefinition{
			{Plugin: "test-plugin"},
		},
	}

	transientErr := &mockTimeoutError{}
	failResult := execution.ObservationResult{
		Status:   values.StatusError,
		RawError: transientErr,
	}

	// Expect 3 calls: Fail -> Fail -> Fail
	mockExec.On("Execute", mock.Anything, mock.Anything).Return(failResult).Times(3)

	result := engine.executeControl(context.Background(), ctrl, 0, nil, nil)

	assert.Equal(t, values.StatusError, result.Status)
	mockExec.AssertNumberOfCalls(t, "Execute", 3)
}

func TestExecuteControl_NonTransientError(t *testing.T) {
	mockExec := new(MockExecutor)
	engine := &Engine{
		executor: mockExec,
		config: ExecutionConfig{
			Parallel: false,
		},
	}

	ctrl := entities.Control{
		ID:      "test-control-perm",
		Retries: 5,
		ObservationDefinitions: []entities.ObservationDefinition{
			{Plugin: "test-plugin"},
		},
	}

	// Permanent error (not transient)
	permResult := execution.ObservationResult{
		Status:   values.StatusError,
		RawError: nil, // or non-transient error
	}

	// Expect 1 call (no retry on permanent error)
	mockExec.On("Execute", mock.Anything, mock.Anything).Return(permResult).Once()

	result := engine.executeControl(context.Background(), ctrl, 0, nil, nil)

	assert.Equal(t, values.StatusError, result.Status)
	mockExec.AssertNumberOfCalls(t, "Execute", 1)
}
