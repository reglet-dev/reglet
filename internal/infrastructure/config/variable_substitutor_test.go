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
