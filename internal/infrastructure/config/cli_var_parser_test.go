package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCLIVar(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantKey     string
		wantValue   interface{}
		wantRaw     string
		wantErr     bool
		errContains string
	}{
		// Basic key=value parsing
		{
			name:      "simple string",
			input:     "key=value",
			wantKey:   "key",
			wantValue: "value",
			wantRaw:   "value",
		},
		{
			name:      "empty value",
			input:     "key=",
			wantKey:   "key",
			wantValue: "",
			wantRaw:   "",
		},
		{
			name:      "value with equals",
			input:     "key=a=b=c",
			wantKey:   "key",
			wantValue: "a=b=c",
			wantRaw:   "a=b=c",
		},
		{
			name:      "nested key",
			input:     "paths.config=/etc/app",
			wantKey:   "paths.config",
			wantValue: "/etc/app",
			wantRaw:   "/etc/app",
		},
		{
			name:      "underscore in key",
			input:     "my_variable=test",
			wantKey:   "my_variable",
			wantValue: "test",
			wantRaw:   "test",
		},

		// Type detection - integers
		{
			name:      "integer zero",
			input:     "port=0",
			wantKey:   "port",
			wantValue: int64(0),
			wantRaw:   "0",
		},
		{
			name:      "positive integer",
			input:     "port=8080",
			wantKey:   "port",
			wantValue: int64(8080),
			wantRaw:   "8080",
		},
		{
			name:      "negative integer",
			input:     "offset=-7",
			wantKey:   "offset",
			wantValue: int64(-7),
			wantRaw:   "-7",
		},
		{
			name:      "leading zero preserved as string",
			input:     "code=007",
			wantKey:   "code",
			wantValue: "007",
			wantRaw:   "007",
		},

		// Type detection - floats
		{
			name:      "positive float",
			input:     "threshold=0.95",
			wantKey:   "threshold",
			wantValue: float64(0.95),
			wantRaw:   "0.95",
		},
		{
			name:      "negative float",
			input:     "offset=-3.14",
			wantKey:   "offset",
			wantValue: float64(-3.14),
			wantRaw:   "-3.14",
		},
		{
			name:      "version string not parsed as float",
			input:     "version=1.0.0",
			wantKey:   "version",
			wantValue: "1.0.0",
			wantRaw:   "1.0.0",
		},

		// Type detection - booleans
		{
			name:      "boolean true",
			input:     "enabled=true",
			wantKey:   "enabled",
			wantValue: true,
			wantRaw:   "true",
		},
		{
			name:      "boolean false",
			input:     "debug=false",
			wantKey:   "debug",
			wantValue: false,
			wantRaw:   "false",
		},
		{
			name:      "TRUE not boolean (case-sensitive)",
			input:     "flag=TRUE",
			wantKey:   "flag",
			wantValue: "TRUE",
			wantRaw:   "TRUE",
		},
		{
			name:      "yes not boolean",
			input:     "confirm=yes",
			wantKey:   "confirm",
			wantValue: "yes",
			wantRaw:   "yes",
		},

		// Error cases
		{
			name:        "no equals sign",
			input:       "keyvalue",
			wantErr:     true,
			errContains: "expected key=value",
		},
		{
			name:        "starts with number",
			input:       "123key=value",
			wantErr:     true,
			errContains: "invalid key",
		},
		{
			name:        "empty key",
			input:       "=value",
			wantErr:     true,
			errContains: "invalid key",
		},
		{
			name:        "special chars in key",
			input:       "key-name=value",
			wantErr:     true,
			errContains: "invalid key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseCLIVar(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantKey, result.Key)
			assert.Equal(t, tt.wantValue, result.Value)
			assert.Equal(t, tt.wantRaw, result.RawValue)
		})
	}
}

func TestDetectValueType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  interface{}
	}{
		// Integers
		{"zero", "0", int64(0)},
		{"positive", "42", int64(42)},
		{"negative", "-100", int64(-100)},
		{"large", "1000000", int64(1000000)},

		// NOT integers (leading zeros)
		{"leading zero", "007", "007"},
		{"zip code", "01234", "01234"},

		// Floats
		{"simple float", "3.14", float64(3.14)},
		{"zero float", "0.0", float64(0.0)},
		{"negative float", "-0.5", float64(-0.5)},

		// NOT floats (version strings)
		{"version", "1.0.0", "1.0.0"},
		{"semver", "2.1.3", "2.1.3"},
		{"date-like", "2024.01.15", "2024.01.15"},

		// Booleans
		{"true", "true", true},
		{"false", "false", false},

		// NOT booleans
		{"TRUE", "TRUE", "TRUE"},
		{"False", "False", "False"},
		{"yes", "yes", "yes"},
		{"no", "no", "no"},
		{"1", "1", int64(1)},
		{"0 as int", "0", int64(0)},

		// Strings
		{"plain text", "hello", "hello"},
		{"with spaces", "hello world", "hello world"},
		{"special chars", "a@b.com", "a@b.com"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectValueType(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSetNestedValue(t *testing.T) {
	tests := []struct {
		name    string
		initial map[string]interface{}
		path    string
		value   interface{}
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name:    "simple key",
			initial: map[string]interface{}{},
			path:    "key",
			value:   "value",
			want:    map[string]interface{}{"key": "value"},
		},
		{
			name:    "nested path creates maps",
			initial: map[string]interface{}{},
			path:    "a.b.c",
			value:   42,
			want: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": 42,
					},
				},
			},
		},
		{
			name: "overwrite existing value",
			initial: map[string]interface{}{
				"key": "old",
			},
			path:  "key",
			value: "new",
			want:  map[string]interface{}{"key": "new"},
		},
		{
			name: "add to existing nested map",
			initial: map[string]interface{}{
				"a": map[string]interface{}{
					"existing": "value",
				},
			},
			path:  "a.new",
			value: "added",
			want: map[string]interface{}{
				"a": map[string]interface{}{
					"existing": "value",
					"new":      "added",
				},
			},
		},
		{
			name: "error when intermediate is not a map",
			initial: map[string]interface{}{
				"a": "scalar",
			},
			path:    "a.b.c",
			value:   "value",
			wantErr: true,
		},
		{
			name:    "error on empty path",
			initial: map[string]interface{}{},
			path:    "",
			value:   "value",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.initial
			err := SetNestedValue(m, tt.path, tt.value)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, m)
		})
	}
}

func TestParseMultipleCLIVars(t *testing.T) {
	tests := []struct {
		name    string
		inputs  []string
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name:   "empty inputs",
			inputs: []string{},
			want:   map[string]interface{}{},
		},
		{
			name:   "single var",
			inputs: []string{"key=value"},
			want:   map[string]interface{}{"key": "value"},
		},
		{
			name:   "multiple vars",
			inputs: []string{"a=1", "b=2", "c=3"},
			want: map[string]interface{}{
				"a": int64(1),
				"b": int64(2),
				"c": int64(3),
			},
		},
		{
			name:   "last wins for same key",
			inputs: []string{"key=first", "key=second", "key=third"},
			want:   map[string]interface{}{"key": "third"},
		},
		{
			name:   "nested paths",
			inputs: []string{"paths.config=/etc", "paths.data=/var"},
			want: map[string]interface{}{
				"paths": map[string]interface{}{
					"config": "/etc",
					"data":   "/var",
				},
			},
		},
		{
			name:   "mixed types",
			inputs: []string{"name=test", "port=8080", "enabled=true", "ratio=0.5"},
			want: map[string]interface{}{
				"name":    "test",
				"port":    int64(8080),
				"enabled": true,
				"ratio":   float64(0.5),
			},
		},
		{
			name:    "error on invalid input",
			inputs:  []string{"valid=ok", "invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMultipleCLIVars(tt.inputs)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
