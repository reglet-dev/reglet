package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	infraconfig "github.com/whiskeyjimbo/reglet/internal/infrastructure/config"
)

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

	loader := infraconfig.NewProfileLoader()
	profile, err := loader.LoadProfileFromReader(strings.NewReader(yaml))
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

	loader := infraconfig.NewProfileLoader()
	_, err := loader.LoadProfileFromReader(strings.NewReader(yaml))
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

	loader := infraconfig.NewProfileLoader()
	profile, err := loader.LoadProfileFromReader(strings.NewReader(yaml))
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

	loader := infraconfig.NewProfileLoader()
	_, err := loader.LoadProfileFromReader(strings.NewReader(yaml))
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

	loader := infraconfig.NewProfileLoader()
	profile, err := loader.LoadProfileFromReader(strings.NewReader(yaml))
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

	loader := infraconfig.NewProfileLoader()
	_, err := loader.LoadProfileFromReader(strings.NewReader(yaml))
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

	loader := infraconfig.NewProfileLoader()
	_, err := loader.LoadProfileFromReader(strings.NewReader(yaml))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must have at least one observation")
}

func TestValidate_MissingPlugin(t *testing.T) {
	// config.Validate checks this
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

	loader := infraconfig.NewProfileLoader()
	profile, err := loader.LoadProfileFromReader(strings.NewReader(yaml))
	require.NoError(t, err)

	err = Validate(profile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin name is required")
}
