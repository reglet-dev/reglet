package wasm

import (
	"testing"

	"github.com/reglet-dev/reglet/internal/domain/capabilities"
	"github.com/stretchr/testify/assert"
)

func TestMatchEnvPattern(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		pattern  string
		expected bool
	}{
		{
			name:     "exact match",
			key:      "AWS_REGION",
			pattern:  "AWS_REGION",
			expected: true,
		},
		{
			name:     "exact match mismatch",
			key:      "AWS_REGION",
			pattern:  "AWS_ACCESS_KEY_ID",
			expected: false,
		},
		{
			name:     "prefix match",
			key:      "AWS_ACCESS_KEY_ID",
			pattern:  "AWS_*",
			expected: true,
		},
		{
			name:     "prefix match 2",
			key:      "AWS_SECRET_ACCESS_KEY",
			pattern:  "AWS_*",
			expected: true,
		},
		{
			name:     "prefix match mismatch",
			key:      "GCP_PROJECT",
			pattern:  "AWS_*",
			expected: false,
		},
		{
			name:     "wildcard match all",
			key:      "ANYTHING",
			pattern:  "*",
			expected: true,
		},
		{
			name:     "empty pattern",
			key:      "ANYTHING",
			pattern:  "",
			expected: false,
		},
		{
			name:     "suffix match (not supported but checking behavior)",
			key:      "MY_AWS_KEY",
			pattern:  "*_KEY",
			expected: false, // current impl only supports prefix or exact
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := capabilities.MatchEnvironmentPattern(tt.key, tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}
