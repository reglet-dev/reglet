package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeCLIVars(t *testing.T) {
	tests := []struct {
		name        string
		profileVars map[string]interface{}
		cliVars     map[string]interface{}
		want        map[string]interface{}
	}{
		{
			name:        "nil profile vars",
			profileVars: nil,
			cliVars:     map[string]interface{}{"key": "value"},
			want:        map[string]interface{}{"key": "value"},
		},
		{
			name:        "nil cli vars",
			profileVars: map[string]interface{}{"key": "value"},
			cliVars:     nil,
			want:        map[string]interface{}{"key": "value"},
		},
		{
			name:        "both nil",
			profileVars: nil,
			cliVars:     nil,
			want:        map[string]interface{}{},
		},
		{
			name:        "cli overrides profile",
			profileVars: map[string]interface{}{"env": "dev"},
			cliVars:     map[string]interface{}{"env": "prod"},
			want:        map[string]interface{}{"env": "prod"},
		},
		{
			name:        "cli adds new key",
			profileVars: map[string]interface{}{"existing": "value"},
			cliVars:     map[string]interface{}{"new": "added"},
			want:        map[string]interface{}{"existing": "value", "new": "added"},
		},
		{
			name: "deep merge nested maps",
			profileVars: map[string]interface{}{
				"paths": map[string]interface{}{
					"config": "/etc/default",
					"data":   "/var/data",
				},
			},
			cliVars: map[string]interface{}{
				"paths": map[string]interface{}{
					"config": "/opt/config",
				},
			},
			want: map[string]interface{}{
				"paths": map[string]interface{}{
					"config": "/opt/config",
					"data":   "/var/data",
				},
			},
		},
		{
			name: "cli scalar overwrites profile map",
			profileVars: map[string]interface{}{
				"server": map[string]interface{}{
					"host": "localhost",
				},
			},
			cliVars: map[string]interface{}{
				"server": "simple-value",
			},
			want: map[string]interface{}{
				"server": "simple-value",
			},
		},
		{
			name: "cli map overwrites profile scalar",
			profileVars: map[string]interface{}{
				"server": "simple-value",
			},
			cliVars: map[string]interface{}{
				"server": map[string]interface{}{
					"host": "localhost",
				},
			},
			want: map[string]interface{}{
				"server": map[string]interface{}{
					"host": "localhost",
				},
			},
		},
		{
			name: "type coercion - cli type wins",
			profileVars: map[string]interface{}{
				"port": "8080",
			},
			cliVars: map[string]interface{}{
				"port": int64(9090),
			},
			want: map[string]interface{}{
				"port": int64(9090),
			},
		},
		{
			name: "does not mutate original profile vars",
			profileVars: map[string]interface{}{
				"key": "original",
			},
			cliVars: map[string]interface{}{
				"key": "modified",
			},
			want: map[string]interface{}{
				"key": "modified",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original profile vars for mutation check
			var originalCopy map[string]interface{}
			if tt.profileVars != nil {
				originalCopy = make(map[string]interface{})
				for k, v := range tt.profileVars {
					originalCopy[k] = v
				}
			}

			got := MergeCLIVars(tt.profileVars, tt.cliVars)

			assert.Equal(t, tt.want, got)

			// Verify original was not mutated
			for k, v := range originalCopy {
				assert.Equal(t, v, tt.profileVars[k], "original profile vars should not be mutated")
			}
		})
	}
}

func TestMergeCLIVars_DoesNotMutateInput(t *testing.T) {
	profileVars := map[string]interface{}{
		"env": "dev",
		"paths": map[string]interface{}{
			"config": "/etc",
		},
	}

	cliVars := map[string]interface{}{
		"env": "prod",
	}

	result := MergeCLIVars(profileVars, cliVars)

	// Result should have merged values
	assert.Equal(t, "prod", result["env"])

	// Original should be unchanged
	assert.Equal(t, "dev", profileVars["env"])
}

func TestDeepCopyMap(t *testing.T) {
	original := map[string]interface{}{
		"simple": "value",
		"nested": map[string]interface{}{
			"inner": "data",
		},
		"array": []interface{}{"a", "b", "c"},
	}

	copied := deepCopyMap(original)

	// Modify the copy
	copied["simple"] = "modified"
	copied["nested"].(map[string]interface{})["inner"] = "changed"
	copied["array"].([]interface{})[0] = "x"

	// Original should be unchanged
	assert.Equal(t, "value", original["simple"])
	assert.Equal(t, "data", original["nested"].(map[string]interface{})["inner"])
	assert.Equal(t, "a", original["array"].([]interface{})[0])
}
