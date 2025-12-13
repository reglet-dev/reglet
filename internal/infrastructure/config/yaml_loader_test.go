package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadProfileFromReader_Valid(t *testing.T) {
	yaml := `
profile:
  name: Test Profile
  version: 1.0.0
controls:
  items:
    - id: ctrl-001
      name: Test Control
      observations:
        - plugin: http
`
	loader := NewProfileLoader()
	profile, err := loader.LoadProfileFromReader(strings.NewReader(yaml))

	require.NoError(t, err)
	assert.Equal(t, "Test Profile", profile.Metadata.Name)
	assert.Equal(t, "1.0.0", profile.Metadata.Version)
	assert.Len(t, profile.Controls.Items, 1)
	assert.Equal(t, "ctrl-001", profile.Controls.Items[0].ID)
}

func TestLoadProfileFromReader_InvalidYAML(t *testing.T) {
	yaml := `invalid yaml: [[[`

	loader := NewProfileLoader()
	_, err := loader.LoadProfileFromReader(strings.NewReader(yaml))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode")
}

func TestLoadProfileFromReader_ValidationFails(t *testing.T) {
	yaml := `
profile:
  name: ""
  version: 1.0.0
controls:
  items: []
`
	loader := NewProfileLoader()
	_, err := loader.LoadProfileFromReader(strings.NewReader(yaml))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestLoadProfileFromReader_AppliesDefaults(t *testing.T) {
	yaml := `
profile:
  name: Test
  version: 1.0.0
controls:
  defaults:
    severity: high
    owner: platform
  items:
    - id: ctrl-001
      name: Test
      observations:
        - plugin: http
`
	loader := NewProfileLoader()
	profile, err := loader.LoadProfileFromReader(strings.NewReader(yaml))

	require.NoError(t, err)
	assert.Equal(t, "high", profile.Controls.Items[0].Severity)
	assert.Equal(t, "platform", profile.Controls.Items[0].Owner)
}
