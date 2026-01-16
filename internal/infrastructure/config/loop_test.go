package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/reglet-dev/reglet/internal/infrastructure/config"
)

func TestLoopConfigParsing(t *testing.T) {
	yamlData := `
controls:
  items:
    - id: test-loop
      name: Test Loop
      observations:
        - loop:
            items: "{{ .vars.files }}"
            as: f
          plugin: file
          config:
            path: "{{ .f.path }}"
          expect:
            - data.exists == true
`
	var profile config.Profile
	err := yaml.Unmarshal([]byte(yamlData), &profile)
	require.NoError(t, err)

	ctrl := profile.Controls.Items[0]
	assert.Equal(t, "test-loop", ctrl.ID)

	obs := ctrl.Observations[0]
	assert.Equal(t, "file", obs.Plugin)
	require.NotNil(t, obs.Loop, "Loop should be parsed")
	assert.Equal(t, "{{ .vars.files }}", obs.Loop.Items)
	assert.Equal(t, "f", obs.Loop.As)

	// Test ToEntity conversion
	entity := obs.ToEntity()
	require.NotNil(t, entity.Loop, "Loop should be converted")
	assert.Equal(t, "{{ .vars.files }}", entity.Loop.Items)
	assert.Equal(t, "f", entity.Loop.As)
}

func TestLoopConfigParsing_SimpleItems(t *testing.T) {
	yamlData := `
controls:
  items:
    - id: simple-loop
      name: Simple Loop
      observations:
        - loop:
            items: "{{ .vars.paths }}"
          plugin: file
          config:
            path: "{{ .loop.item }}"
`
	var profile config.Profile
	err := yaml.Unmarshal([]byte(yamlData), &profile)
	require.NoError(t, err)

	obs := profile.Controls.Items[0].Observations[0]
	require.NotNil(t, obs.Loop, "Loop should be parsed")
	assert.Equal(t, "{{ .vars.paths }}", obs.Loop.Items)
	assert.Equal(t, "", obs.Loop.As) // No 'as' specified
}
