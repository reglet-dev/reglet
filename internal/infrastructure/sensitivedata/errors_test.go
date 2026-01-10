package sensitivedata

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafeError(t *testing.T) {
	provider := NewProvider()
	provider.Track("very-secret-token")

	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "No secret",
			err:      errors.New("something failed"),
			expected: "something failed",
		},
		{
			name:     "Detailed error with secret",
			err:      errors.New("API call failed with token: very-secret-token"),
			expected: "API call failed with token: [REDACTED]",
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SafeError(tt.err, provider)
			if tt.err == nil {
				assert.NoError(t, got)
			} else {
				assert.EqualError(t, got, tt.expected)
			}
		})
	}
}
