package engine

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"time"

	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/stretchr/testify/assert"
)

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		name         string
		strategy     entities.BackoffType
		attempt      int
		initialDelay time.Duration
		maxDelay     time.Duration
		expected     time.Duration
	}{
		{
			name:         "None strategy",
			strategy:     entities.BackoffNone,
			attempt:      1,
			initialDelay: time.Second,
			maxDelay:     10 * time.Second,
			expected:     time.Second, // Always returns initialDelay
		},
		{
			name:         "Linear attempt 1",
			strategy:     entities.BackoffLinear,
			attempt:      1,
			initialDelay: time.Second,
			maxDelay:     10 * time.Second,
			expected:     time.Second,
		},
		{
			name:         "Linear attempt 3",
			strategy:     entities.BackoffLinear,
			attempt:      3,
			initialDelay: time.Second,
			maxDelay:     10 * time.Second,
			expected:     3 * time.Second,
		},
		{
			name:         "Linear capped",
			strategy:     entities.BackoffLinear,
			attempt:      20,
			initialDelay: time.Second,
			maxDelay:     10 * time.Second,
			expected:     10 * time.Second,
		},
		{
			name:         "Exponential attempt 1",
			strategy:     entities.BackoffExponential,
			attempt:      1,
			initialDelay: time.Second,
			maxDelay:     100 * time.Second,
			expected:     2 * time.Second, // 2^1 * 1s
		},
		{
			name:         "Exponential attempt 3",
			strategy:     entities.BackoffExponential,
			attempt:      3,
			initialDelay: time.Second,
			maxDelay:     100 * time.Second,
			expected:     8 * time.Second, // 2^3 * 1s
		},
		{
			name:         "Exponential capped",
			strategy:     entities.BackoffExponential,
			attempt:      10,
			initialDelay: time.Second,
			maxDelay:     5 * time.Second,
			expected:     5 * time.Second,
		},
		{
			name:         "Exponential huge attempt (overflow protection)",
			strategy:     entities.BackoffExponential,
			attempt:      100,
			initialDelay: time.Second,
			maxDelay:     time.Minute,
			expected:     time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateBackoff(tt.strategy, tt.attempt, tt.initialDelay, tt.maxDelay)
			assert.Equal(t, tt.expected, got)
		})
	}
}

type mockTimeoutError struct{}

func (e *mockTimeoutError) Error() string   { return "timeout" }
func (e *mockTimeoutError) Timeout() bool   { return true }
func (e *mockTimeoutError) Temporary() bool { return true }

func TestIsTransientError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "Context canceled",
			err:      context.Canceled,
			expected: false,
		},
		{
			name:     "Context deadline",
			err:      context.DeadlineExceeded,
			expected: false,
		},
		{
			name:     "Generic error",
			err:      errors.New("something bad"),
			expected: false,
		},
		{
			name:     "Net timeout",
			err:      &mockTimeoutError{},
			expected: true,
		},
		{
			name:     "Syscall ECONNRESET",
			err:      syscall.ECONNRESET,
			expected: true,
		},
		{
			name:     "Syscall ECONNREFUSED",
			err:      syscall.ECONNREFUSED,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTransientError(tt.err)
			assert.Equal(t, tt.expected, got)
		})
	}
}
