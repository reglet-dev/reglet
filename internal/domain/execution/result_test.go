package execution_test

import (
	"strings"
	"testing"

	"github.com/reglet-dev/reglet/internal/domain/execution"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTruncateEvidence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		input         map[string]interface{}
		limit         int
		wantTruncated bool
		checkContent  func(t *testing.T, original, result map[string]interface{})
	}{
		{
			name: "under limit",
			input: map[string]interface{}{
				"key": "value",
			},
			limit:         1000,
			wantTruncated: false,
			checkContent: func(t *testing.T, original, result map[string]interface{}) {
				assert.Equal(t, original, result)
			},
		},
		{
			name: "no limit (zero)",
			input: map[string]interface{}{
				"key": strings.Repeat("a", 100),
			},
			limit:         0,
			wantTruncated: false,
			checkContent: func(t *testing.T, original, result map[string]interface{}) {
				assert.Equal(t, original, result)
			},
		},
		{
			name: "over limit - large string",
			input: map[string]interface{}{
				"small": "val",
				"large": strings.Repeat("a", 1000),
			},
			limit:         500,
			wantTruncated: true,
			checkContent: func(t *testing.T, original, result map[string]interface{}) {
				assert.Equal(t, "val", result["small"])
				large, ok := result["large"].(string)
				require.True(t, ok, "large field should be string")
				assert.True(t, len(large) < 1000, "large field should be truncated")
				assert.Contains(t, large, "[TRUNCATED]")
			},
		},
		{
			name: "over limit - large object",
			input: map[string]interface{}{
				"complex": map[string]interface{}{
					"data": strings.Repeat("b", 1000),
				},
			},
			limit:         500,
			wantTruncated: true,
			checkContent: func(t *testing.T, original, result map[string]interface{}) {
				complexVal, ok := result["complex"].(map[string]interface{})
				if ok {
					// Either specific fields inside were truncated or the whole object replaced
					// simpler heuristic in implementation replaces large objects with map
					assert.Contains(t, complexVal, "_truncated")
				} else {
					// It's possible the implementation structure changed, verify it's not original
					assert.NotEqual(t, original["complex"], result["complex"])
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Make a copy to ensure original is not mutated by function
			originalCopy := make(map[string]interface{})
			for k, v := range tt.input {
				originalCopy[k] = v
			}

			// Use GreedyTruncator
			truncator := &execution.GreedyTruncator{}
			truncated, meta, err := truncator.Truncate(tt.input, tt.limit)
			require.NoError(t, err)

			if tt.wantTruncated {
				require.NotNil(t, meta)
				assert.True(t, meta.Truncated)
				assert.Greater(t, meta.OriginalSize, tt.limit)
				assert.Equal(t, tt.limit, meta.TruncatedAt)
				assert.NotEmpty(t, meta.Reason)
			} else {
				assert.Nil(t, meta)
			}

			tt.checkContent(t, originalCopy, truncated)

			// Verify original input was NOT mutated
			assert.Equal(t, originalCopy, tt.input)
		})
	}
}
