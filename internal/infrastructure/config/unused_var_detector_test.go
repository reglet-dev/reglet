package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindUnusedVars(t *testing.T) {
	tests := []struct {
		name     string
		cliVars  map[string]interface{}
		content  string
		expected []string
	}{
		{
			name:     "no cli vars",
			cliVars:  nil,
			content:  "{{ .vars.foo }}",
			expected: nil,
		},
		{
			name:     "empty cli vars",
			cliVars:  map[string]interface{}{},
			content:  "{{ .vars.foo }}",
			expected: nil,
		},
		{
			name:     "all vars used",
			cliVars:  map[string]interface{}{"env": "prod"},
			content:  "environment: {{ .vars.env }}",
			expected: nil,
		},
		{
			name:     "one unused var",
			cliVars:  map[string]interface{}{"env": "prod", "typo": "value"},
			content:  "environment: {{ .vars.env }}",
			expected: []string{"typo"},
		},
		{
			name:     "all vars unused",
			cliVars:  map[string]interface{}{"foo": "bar", "baz": "qux"},
			content:  "no variables here",
			expected: []string{"foo", "baz"},
		},
		{
			name: "nested var referenced",
			cliVars: map[string]interface{}{
				"paths": map[string]interface{}{
					"config": "/etc",
				},
			},
			content:  "config: {{ .vars.paths.config }}",
			expected: nil,
		},
		{
			name: "nested var top-level referenced",
			cliVars: map[string]interface{}{
				"paths": map[string]interface{}{
					"config": "/etc",
				},
			},
			content:  "paths: {{ .vars.paths }}",
			expected: nil,
		},
		{
			name:     "multiple references same var",
			cliVars:  map[string]interface{}{"env": "prod"},
			content:  "{{ .vars.env }} and {{ .vars.env }} again",
			expected: nil,
		},
		{
			name:     "whitespace in template",
			cliVars:  map[string]interface{}{"env": "prod"},
			content:  "{{  .vars.env  }}",
			expected: nil,
		},
		{
			name:     "mixed used and unused",
			cliVars:  map[string]interface{}{"used1": "a", "unused1": "b", "used2": "c", "unused2": "d"},
			content:  "{{ .vars.used1 }} and {{ .vars.used2 }}",
			expected: []string{"unused1", "unused2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindUnusedVars(tt.cliVars, tt.content)

			if tt.expected == nil {
				assert.Nil(t, got)
				return
			}

			// Compare as sets since order doesn't matter
			assert.ElementsMatch(t, tt.expected, got)
		})
	}
}

func TestExtractVarReferences(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected map[string]bool
	}{
		{
			name:     "simple var",
			content:  "{{ .vars.foo }}",
			expected: map[string]bool{"foo": true},
		},
		{
			name:     "nested var",
			content:  "{{ .vars.paths.config }}",
			expected: map[string]bool{"paths.config": true, "paths": true},
		},
		{
			name:     "multiple vars",
			content:  "{{ .vars.a }} and {{ .vars.b }}",
			expected: map[string]bool{"a": true, "b": true},
		},
		{
			name:     "no vars",
			content:  "just plain text",
			expected: map[string]bool{},
		},
		{
			name:     "secret not matched",
			content:  "{{ secret \"key\" }}",
			expected: map[string]bool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractVarReferences(tt.content)
			assert.Equal(t, tt.expected, got)
		})
	}
}
