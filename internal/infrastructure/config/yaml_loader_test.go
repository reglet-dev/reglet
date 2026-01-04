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
          config:
            url: http://example.com
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

func TestLoadProfileFromReader_LoadsRawProfile(t *testing.T) {
	// ProfileLoader now just loads the raw profile without validation
	// The application layer is responsible for compiling (validating + applying defaults)
	yaml := `
profile:
  name: ""
  version: 1.0.0
controls:
  items: []
`
	loader := NewProfileLoader()
	profile, err := loader.LoadProfileFromReader(strings.NewReader(yaml))

	// Should successfully load even with invalid data
	// Validation happens in ProfileCompiler, not ProfileLoader
	require.NoError(t, err)
	assert.NotNil(t, profile)
	assert.Empty(t, profile.Metadata.Name) // Empty name is loaded as-is
}

func TestLoadProfileFromReader_LoadsDefaultsWithoutApplying(t *testing.T) {
	// ProfileLoader now just loads the raw profile
	// Defaults are NOT applied by the loader - that's done by ProfileCompiler
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
          config:
            url: http://example.com
`
	loader := NewProfileLoader()
	profile, err := loader.LoadProfileFromReader(strings.NewReader(yaml))

	require.NoError(t, err)
	// Defaults are loaded but NOT applied to controls
	assert.NotNil(t, profile.Controls.Defaults)
	assert.Equal(t, "high", profile.Controls.Defaults.Severity)
	assert.Equal(t, "platform", profile.Controls.Defaults.Owner)

	// Control should NOT have defaults applied yet
	assert.Empty(t, profile.Controls.Items[0].Severity)
	assert.Empty(t, profile.Controls.Items[0].Owner)
}
