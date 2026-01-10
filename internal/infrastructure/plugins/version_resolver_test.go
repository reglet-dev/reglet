package plugins_test

import (
	"testing"

	"github.com/reglet-dev/reglet/internal/infrastructure/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSemverResolver_Resolve(t *testing.T) {
	t.Parallel()

	resolver := plugins.NewSemverResolver()

	tests := []struct {
		name       string
		constraint string
		available  []string
		expected   string
		wantErr    bool
	}{
		{
			name:       "exact match",
			constraint: "1.0.0",
			available:  []string{"0.9.0", "1.0.0", "1.1.0"},
			expected:   "1.0.0",
		},
		{
			name:       "caret range",
			constraint: "^1.0",
			available:  []string{"0.9", "1.0.0", "1.0.2", "1.1.0", "2.0.0"},
			expected:   "1.1.0",
		},
		{
			name:       "tilde range",
			constraint: "~1.2.0",
			available:  []string{"1.2.0", "1.2.5", "1.3.0"},
			expected:   "1.2.5",
		},
		{
			name:       "latest",
			constraint: "latest",
			available:  []string{"1.0.0", "2.0.0", "1.5.0"},
			expected:   "2.0.0",
		},
		{
			name:       "no match",
			constraint: "^2.0",
			available:  []string{"1.0.0", "1.9.9"},
			wantErr:    true,
		},
		{
			name:       "invalid constraint",
			constraint: "invalid",
			available:  []string{"1.0.0"},
			wantErr:    true,
		},
		{
			name:       "mixed valid/invalid available",
			constraint: "^1.0",
			available:  []string{"1.0.0", "invalid-v", "1.1.0"},
			expected:   "1.1.0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolver.Resolve(tc.constraint, tc.available)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, got)
			}
		})
	}
}
