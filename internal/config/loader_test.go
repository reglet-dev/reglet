package config

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadProfileFromReader_Valid(t *testing.T) {
	yaml := `
profile:
  name: test-profile
  version: 1.0.0
  description: Test profile

vars:
  environment: production
  config_path: /etc/app/config.yaml

controls:
  items:
    - id: test-control
      name: Test Control
      observations:
        - plugin: file
          config:
            path: /etc/config
            mode: exists
`

	profile, err := LoadProfileFromReader(strings.NewReader(yaml))
	require.NoError(t, err)
	require.NotNil(t, profile)

	// Verify metadata
	assert.Equal(t, "test-profile", profile.Metadata.Name)
	assert.Equal(t, "1.0.0", profile.Metadata.Version)
	assert.Equal(t, "Test profile", profile.Metadata.Description)

	// Verify vars
	assert.Len(t, profile.Vars, 2)
	assert.Equal(t, "production", profile.Vars["environment"])
	assert.Equal(t, "/etc/app/config.yaml", profile.Vars["config_path"])

	// Verify controls
	require.Len(t, profile.Controls.Items, 1)
	assert.Equal(t, "test-control", profile.Controls.Items[0].ID)
	assert.Equal(t, "Test Control", profile.Controls.Items[0].Name)

	// Verify observations
	require.Len(t, profile.Controls.Items[0].Observations, 1)
	assert.Equal(t, "file", profile.Controls.Items[0].Observations[0].Plugin)
	assert.Equal(t, "/etc/config", profile.Controls.Items[0].Observations[0].Config["path"])
}

func TestLoadProfileFromReader_InvalidYAML(t *testing.T) {
	yaml := `
profile:
  name: test
  invalid yaml here: [
`

	_, err := LoadProfileFromReader(strings.NewReader(yaml))
	require.Error(t, err)
	// The new infrastructure loader returns "failed to decode profile YAML"
	assert.Contains(t, err.Error(), "failed to decode")
}

func TestApplyDefaults(t *testing.T) {
	yaml := `
profile:
  name: test-profile
  version: 1.0.0

controls:
  defaults:
    severity: medium
    owner: security-team
    tags: [default-tag]
    timeout: 30s

  items:
    - id: control-1
      name: Control 1
      observations:
        - plugin: file
          config:
            path: /etc/test

    - id: control-2
      name: Control 2
      severity: critical
      tags: [custom-tag]
      observations:
        - plugin: file
          config:
            path: /etc/test2
`

	// This now uses the infrastructure loader which calls ApplyDefaults internally
	profile, err := LoadProfileFromReader(strings.NewReader(yaml))
	require.NoError(t, err)

	// First control should have defaults applied
	ctrl1 := profile.Controls.Items[0]
	assert.Equal(t, "medium", ctrl1.Severity)
	assert.Equal(t, "security-team", ctrl1.Owner)
	assert.Equal(t, 30*time.Second, ctrl1.Timeout)
	assert.Contains(t, ctrl1.Tags, "default-tag")

	// Second control should override severity but merge tags
	ctrl2 := profile.Controls.Items[1]
	assert.Equal(t, "critical", ctrl2.Severity)   // Overridden
	assert.Equal(t, "security-team", ctrl2.Owner) // From default
	assert.Contains(t, ctrl2.Tags, "default-tag") // From default
	assert.Contains(t, ctrl2.Tags, "custom-tag")  // From control
}

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

	profile, err := LoadProfileFromReader(strings.NewReader(yaml))
	require.NoError(t, err)

	err = SubstituteVariables(profile)
	require.NoError(t, err)

	// Verify substitution in description
	assert.Equal(t, "Checking file in production", profile.Controls.Items[0].Description)

	// Verify substitution in observation config
	assert.Equal(t, "/tmp/test.txt", profile.Controls.Items[0].Observations[0].Config["path"])
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

	profile, err := LoadProfileFromReader(strings.NewReader(yaml))
	require.NoError(t, err)

	err = SubstituteVariables(profile)
	require.NoError(t, err)

	// Verify nested variable substitution
	assert.Equal(t, "/etc/app/config.yaml", profile.Controls.Items[0].Observations[0].Config["path"])
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

	profile, err := LoadProfileFromReader(strings.NewReader(yaml))
	require.NoError(t, err)

	err = SubstituteVariables(profile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "variable not found: missing_var")
}

func TestValidate_Valid(t *testing.T) {
	yaml := `
profile:
  name: test-profile
  version: 1.0.0

controls:
  items:
    - id: test-control
      name: Test Control
      observations:
        - plugin: file
          config:
            path: /etc/test
`

	profile, err := LoadProfileFromReader(strings.NewReader(yaml))
	require.NoError(t, err)

	err = Validate(profile)
	assert.NoError(t, err)
}

func TestValidate_MissingName(t *testing.T) {
	// Infrastructure loader validates profile name is required, so LoadProfileFromReader fails first
	yaml := `
profile:
  version: 1.0.0

controls:
  items:
    - id: test-control
      name: Test Control
      observations:
        - plugin: file
          config:
            path: /etc/test
`

	_, err := LoadProfileFromReader(strings.NewReader(yaml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name cannot be empty")
}

func TestValidate_InvalidVersion(t *testing.T) {
	// Infrastructure loader validates existence, Validate checks format
	yaml := `
profile:
  name: test-profile
  version: invalid

controls:
  items:
    - id: test-control
      name: Test Control
      observations:
        - plugin: file
          config:
            path: /etc/test
`

	profile, err := LoadProfileFromReader(strings.NewReader(yaml))
	require.NoError(t, err)

	err = Validate(profile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version")
	assert.Contains(t, err.Error(), "not valid")
}

func TestValidate_MissingControlID(t *testing.T) {
	// Infrastructure loader validates Control ID
	yaml := `
profile:
  name: test-profile
  version: 1.0.0

controls:
  items:
    - name: Test Control
      observations:
        - plugin: file
          config:
            path: /etc/test
`

	_, err := LoadProfileFromReader(strings.NewReader(yaml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "control ID cannot be empty")
}

func TestValidate_InvalidControlID(t *testing.T) {
	// Infrastructure loader DOES NOT validate format strictly, but Validate DOES
	yaml := `
profile:
  name: test-profile
  version: 1.0.0

controls:
  items:
    - id: "invalid id with spaces"
      name: Test Control
      observations:
        - plugin: file
          config:
            path: /etc/test
`

	profile, err := LoadProfileFromReader(strings.NewReader(yaml))
	require.NoError(t, err)

	err = Validate(profile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "control ID")
	assert.Contains(t, err.Error(), "invalid")
}

func TestValidate_DuplicateControlIDs(t *testing.T) {
	// Infrastructure loader checks for duplicates
	yaml := `
profile:
  name: test-profile
  version: 1.0.0

controls:
  items:
    - id: test-control
      name: Test Control 1
      observations:
        - plugin: file
          config:
            path: /etc/test1

    - id: test-control
      name: Test Control 2
      observations:
        - plugin: file
          config:
            path: /etc/test2
`

	_, err := LoadProfileFromReader(strings.NewReader(yaml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate control ID")
}

func TestValidate_NoObservations(t *testing.T) {
	// Infrastructure loader checks this
	yaml := `
profile:
  name: test-profile
  version: 1.0.0

controls:
  items:
    - id: test-control
      name: Test Control
      observations: []
`

	_, err := LoadProfileFromReader(strings.NewReader(yaml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must have at least one observation")
}

func TestValidate_MissingPlugin(t *testing.T) {
	// Infrastructure loader does NOT check this (Observation.Validate not called in loader?)
	// Actually entity.Control.Validate calls nothing on Observation struct as there is no Validate method on Observation.
	// But config.Validate DOES.
	yaml := `
profile:
  name: test-profile
  version: 1.0.0

controls:
  items:
    - id: test-control
      name: Test Control
      observations:
        - config:
            path: /etc/test
`

	profile, err := LoadProfileFromReader(strings.NewReader(yaml))
	require.NoError(t, err)

	err = Validate(profile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin name is required")
}
