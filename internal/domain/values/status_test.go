package values

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Status_Precedence(t *testing.T) {
	tests := []struct {
		status     Status
		precedence int
	}{
		{StatusFail, 3},
		{StatusError, 2},
		{StatusSkipped, 1},
		{StatusPass, 0},
		{Status("unknown"), -1},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			assert.Equal(t, tt.precedence, tt.status.Precedence())
		})
	}

	// Verify ordering
	assert.True(t, StatusFail.Precedence() > StatusError.Precedence())
	assert.True(t, StatusError.Precedence() > StatusSkipped.Precedence())
	assert.True(t, StatusSkipped.Precedence() > StatusPass.Precedence())
}

func Test_Status_IsFailure(t *testing.T) {
	assert.True(t, StatusFail.IsFailure())
	assert.True(t, StatusError.IsFailure())
	assert.False(t, StatusPass.IsFailure())
	assert.False(t, StatusSkipped.IsFailure())
}

func Test_Status_IsSuccess(t *testing.T) {
	assert.True(t, StatusPass.IsSuccess())
	assert.False(t, StatusFail.IsSuccess())
	assert.False(t, StatusError.IsSuccess())
	assert.False(t, StatusSkipped.IsSuccess())
}

func Test_Status_IsSkipped(t *testing.T) {
	assert.True(t, StatusSkipped.IsSkipped())
	assert.False(t, StatusPass.IsSkipped())
	assert.False(t, StatusFail.IsSkipped())
	assert.False(t, StatusError.IsSkipped())
}

func Test_Status_Validate(t *testing.T) {
	validStatuses := []Status{StatusPass, StatusFail, StatusError, StatusSkipped}

	for _, s := range validStatuses {
		t.Run(string(s), func(t *testing.T) {
			assert.NoError(t, s.Validate())
		})
	}

	invalid := Status("invalid")
	assert.Error(t, invalid.Validate())
}
