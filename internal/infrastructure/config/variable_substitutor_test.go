package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubstituteVariables_Simple(t *testing.T) {
	yaml := `
profile:
  name: test-profile
  version: 1.0.0

vars:
  test_file: /tmp/test.txt
  environment: production

controls:
  items:
    - id: test-control
      name: Test Control
      description: "Checking file in {{ .vars.environment }}"
      observations:
        - plugin: file
          config:
            path: "{{ .vars.test_file }}"
            mode: exists
`

	loader := NewProfileLoader()
	profile, err := loader.LoadProfileFromReader(strings.NewReader(yaml))
	require.NoError(t, err)

	substitutor := NewVariableSubstitutor(nil)
	err = substitutor.Substitute(profile)
	require.NoError(t, err)

	// Verify substitution in description
	assert.Equal(t, "Checking file in production", profile.Controls.Items[0].Description)

	// Verify substitution in observation config
	assert.Equal(t, "/tmp/test.txt", profile.Controls.Items[0].ObservationDefinitions[0].Config["path"])
}

func TestSubstituteVariables_Nested(t *testing.T) {
	yaml := `
profile:
  name: test-profile
  version: 1.0.0

vars:
  paths:
    config: /etc/app/config.yaml
    data: /var/lib/app/data

controls:
  items:
    - id: test-control
      name: Test Control
      observations:
        - plugin: file
          config:
            path: "{{ .vars.paths.config }}"
`

	loader := NewProfileLoader()
	profile, err := loader.LoadProfileFromReader(strings.NewReader(yaml))
	require.NoError(t, err)

	substitutor := NewVariableSubstitutor(nil)
	err = substitutor.Substitute(profile)
	require.NoError(t, err)

	// Verify nested variable substitution
	assert.Equal(t, "/etc/app/config.yaml", profile.Controls.Items[0].ObservationDefinitions[0].Config["path"])
}

func TestSubstituteVariables_Missing(t *testing.T) {
	yaml := `
profile:
  name: test-profile
  version: 1.0.0

vars:
  existing_var: value

controls:
  items:
    - id: test-control
      name: Test Control
      observations:
        - plugin: file
          config:
            path: "{{ .vars.missing_var }}"
`

	loader := NewProfileLoader()
	profile, err := loader.LoadProfileFromReader(strings.NewReader(yaml))
	require.NoError(t, err)

	substitutor := NewVariableSubstitutor(nil)
	err = substitutor.Substitute(profile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "variable not found: missing_var")
}

// MockSecretResolver for testing
type MockSecretResolver struct {
	secrets map[string]string
}

func (m *MockSecretResolver) Resolve(name string) (string, error) {
	if val, ok := m.secrets[name]; ok {
		return val, nil
	}
	return "", assert.AnError
}

func TestSubstituteVariables_Secrets(t *testing.T) {
	yaml := `
profile:
  name: test-profile
  version: 1.0.0

controls:
  items:
    - id: test-control
      name: Secret Control
      observations:
        - plugin: http
          config:
            token: '{{ secret "api_key" }}'
            nested:
              key: '{{ secret "db_pass" }}'
`

	loader := NewProfileLoader()
	profile, err := loader.LoadProfileFromReader(strings.NewReader(yaml))
	require.NoError(t, err)

	resolver := &MockSecretResolver{
		secrets: map[string]string{
			"api_key": "super-secret-token",
			"db_pass": "secure-password",
		},
	}

	substitutor := NewVariableSubstitutor(resolver)
	err = substitutor.Substitute(profile)
	require.NoError(t, err)

	// Verify secret substitution
	assert.Equal(t, "super-secret-token", profile.Controls.Items[0].ObservationDefinitions[0].Config["token"])

	// Verify nested substitution
	nested := profile.Controls.Items[0].ObservationDefinitions[0].Config["nested"].(map[string]interface{})
	assert.Equal(t, "secure-password", nested["key"])
}

// TestLookupVar_AllNumericTypes verifies robust handling of all numeric types.
// This tests the fix for incomplete reflection logic that only handled int, int64, float64.
func TestLookupVar_AllNumericTypes(t *testing.T) {
	tests := []struct {
		name     string
		vars     map[string]interface{}
		path     string
		expected interface{}
	}{
		{
			name:     "int",
			vars:     map[string]interface{}{"value": int(42)},
			path:     "value",
			expected: int64(42),
		},
		{
			name:     "int8",
			vars:     map[string]interface{}{"value": int8(127)},
			path:     "value",
			expected: int64(127),
		},
		{
			name:     "int16",
			vars:     map[string]interface{}{"value": int16(32767)},
			path:     "value",
			expected: int64(32767),
		},
		{
			name:     "int32",
			vars:     map[string]interface{}{"value": int32(2147483647)},
			path:     "value",
			expected: int64(2147483647),
		},
		{
			name:     "int64",
			vars:     map[string]interface{}{"value": int64(9223372036854775807)},
			path:     "value",
			expected: int64(9223372036854775807),
		},
		{
			name:     "uint",
			vars:     map[string]interface{}{"value": uint(42)},
			path:     "value",
			expected: uint64(42),
		},
		{
			name:     "uint8",
			vars:     map[string]interface{}{"value": uint8(255)},
			path:     "value",
			expected: uint64(255),
		},
		{
			name:     "uint16",
			vars:     map[string]interface{}{"value": uint16(65535)},
			path:     "value",
			expected: uint64(65535),
		},
		{
			name:     "uint32",
			vars:     map[string]interface{}{"value": uint32(4294967295)},
			path:     "value",
			expected: uint64(4294967295),
		},
		{
			name:     "uint64",
			vars:     map[string]interface{}{"value": uint64(18446744073709551615)},
			path:     "value",
			expected: uint64(18446744073709551615),
		},
		{
			name:     "float32",
			vars:     map[string]interface{}{"value": float32(3.14)},
			path:     "value",
			expected: float64(3.140000104904175), // float32 precision loss
		},
		{
			name:     "float64",
			vars:     map[string]interface{}{"value": float64(3.14159265359)},
			path:     "value",
			expected: float64(3.14159265359),
		},
		{
			name:     "bool",
			vars:     map[string]interface{}{"value": true},
			path:     "value",
			expected: true,
		},
		{
			name:     "string",
			vars:     map[string]interface{}{"value": "hello"},
			path:     "value",
			expected: "hello",
		},
		{
			name: "nested_numeric",
			vars: map[string]interface{}{
				"config": map[string]interface{}{
					"port": int16(8080),
				},
			},
			path:     "config.port",
			expected: int64(8080),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := lookupVar(tt.vars, tt.path)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestSubstituteVariables_NumericTypes verifies numeric types work in real substitution.
func TestSubstituteVariables_NumericTypes(t *testing.T) {
	yaml := `
profile:
  name: test-profile
  version: 1.0.0

vars:
  port: 8080
  timeout_ms: 5000
  retry_count: 3
  enabled: true

controls:
  items:
    - id: test-control
      name: Test Control
      description: "Port {{ .vars.port }}, timeout {{ .vars.timeout_ms }}ms"
      observations:
        - plugin: tcp
          config:
            port: "{{ .vars.port }}"
            timeout: "{{ .vars.timeout_ms }}"
            retries: "{{ .vars.retry_count }}"
            enabled: "{{ .vars.enabled }}"
`

	loader := NewProfileLoader()
	profile, err := loader.LoadProfileFromReader(strings.NewReader(yaml))
	require.NoError(t, err)

	substitutor := NewVariableSubstitutor(nil)
	err = substitutor.Substitute(profile)
	require.NoError(t, err)

	// Verify numeric substitution in description (converts to string)
	assert.Contains(t, profile.Controls.Items[0].Description, "8080")
	assert.Contains(t, profile.Controls.Items[0].Description, "5000")

	// Verify numeric values are substituted as strings (template syntax always produces strings)
	config := profile.Controls.Items[0].ObservationDefinitions[0].Config
	assert.Equal(t, "8080", config["port"])
	assert.Equal(t, "5000", config["timeout"])
	assert.Equal(t, "3", config["retries"])
	assert.Equal(t, "true", config["enabled"])
}

// TestSubstituteVariables_ExpectExpressions verifies variable substitution in expect expressions.
// This was a bug fix: expect expressions with {{ .vars.xxx }} were not being substituted.
func TestSubstituteVariables_ExpectExpressions(t *testing.T) {
	yaml := `
profile:
  name: test-profile
  version: 1.0.0

vars:
  max_response_time_ms: 1000
  min_file_size: 100

controls:
  items:
    - id: test-control
      name: Test Control
      observations:
        - plugin: http
          config:
            url: "http://example.com"
          expect:
            - "data.response_time_ms < {{ .vars.max_response_time_ms }}"
            - "data.size > {{ .vars.min_file_size }}"
`

	loader := NewProfileLoader()
	profile, err := loader.LoadProfileFromReader(strings.NewReader(yaml))
	require.NoError(t, err)

	substitutor := NewVariableSubstitutor(nil)
	err = substitutor.Substitute(profile)
	require.NoError(t, err)

	// Verify expect expressions have variables substituted
	expects := profile.Controls.Items[0].ObservationDefinitions[0].Expect
	assert.Equal(t, "data.response_time_ms < 1000", expects[0])
	assert.Equal(t, "data.size > 100", expects[1])
}
