package scaffold

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitOptions_Validate(t *testing.T) {
	tests := []struct {
		name    string
		opts    InitOptions
		wantErr string
	}{
		{
			name: "valid options",
			opts: InitOptions{
				ProfileName: "my-profile",
				Plugins:     []string{"file"},
				OutputPath:  "./test.yaml",
			},
			wantErr: "",
		},
		{
			name: "empty profile name",
			opts: InitOptions{
				ProfileName: "",
				Plugins:     []string{"file"},
				OutputPath:  "./test.yaml",
			},
			wantErr: "profile name is required",
		},
		{
			name: "invalid profile name - starts with number",
			opts: InitOptions{
				ProfileName: "1profile",
				Plugins:     []string{"file"},
				OutputPath:  "./test.yaml",
			},
			wantErr: "invalid profile name",
		},
		{
			name: "invalid profile name - special chars",
			opts: InitOptions{
				ProfileName: "my profile!",
				Plugins:     []string{"file"},
				OutputPath:  "./test.yaml",
			},
			wantErr: "invalid profile name",
		},
		{
			name: "no plugins",
			opts: InitOptions{
				ProfileName: "my-profile",
				Plugins:     []string{},
				OutputPath:  "./test.yaml",
			},
			wantErr: "at least one plugin must be selected",
		},
		{
			name: "invalid plugin",
			opts: InitOptions{
				ProfileName: "my-profile",
				Plugins:     []string{"file", "invalid"},
				OutputPath:  "./test.yaml",
			},
			wantErr: "unknown plugin 'invalid'",
		},
		{
			name: "empty output path",
			opts: InitOptions{
				ProfileName: "my-profile",
				Plugins:     []string{"file"},
				OutputPath:  "",
			},
			wantErr: "output path is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateProfileName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid lowercase", "myprofile", false},
		{"valid with hyphen", "my-profile", false},
		{"valid with underscore", "my_profile", false},
		{"valid with numbers", "profile123", false},
		{"valid mixed", "My-Profile_123", false},
		{"empty", "", true},
		{"starts with number", "1profile", true},
		{"starts with hyphen", "-profile", true},
		{"has spaces", "my profile", true},
		{"has special chars", "my@profile", true},
		{"too long", strings.Repeat("a", 65), true},
		{"max length", strings.Repeat("a", 64), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProfileName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProfileGenerator_Generate(t *testing.T) {
	gen := NewProfileGenerator()

	t.Run("single plugin", func(t *testing.T) {
		opts := &InitOptions{
			ProfileName: "test-profile",
			Plugins:     []string{"file"},
			OutputPath:  "./test.yaml",
		}

		result, err := gen.Generate(opts)
		require.NoError(t, err)
		require.NotNil(t, result)

		content := string(result.ProfileContent)
		assert.Contains(t, content, "name: test-profile")
		assert.Contains(t, content, "- file")
		assert.Contains(t, content, "file-exists-check")
		assert.Contains(t, content, "data.exists")
		assert.Nil(t, result.ConfigContent)
		assert.Equal(t, "./test.yaml", result.ProfilePath)
	})

	t.Run("multiple plugins", func(t *testing.T) {
		opts := &InitOptions{
			ProfileName: "multi-plugin",
			Plugins:     []string{"file", "http"},
			OutputPath:  "./multi.yaml",
		}

		result, err := gen.Generate(opts)
		require.NoError(t, err)

		content := string(result.ProfileContent)
		assert.Contains(t, content, "- file")
		assert.Contains(t, content, "- http")
		assert.Contains(t, content, "file-exists-check")
		assert.Contains(t, content, "http-health-check")
	})

	t.Run("with config", func(t *testing.T) {
		opts := &InitOptions{
			ProfileName: "with-config",
			Plugins:     []string{"file", "http"},
			OutputPath:  "./test.yaml",
			WithConfig:  true,
		}

		result, err := gen.Generate(opts)
		require.NoError(t, err)
		require.NotNil(t, result.ConfigContent)

		configContent := string(result.ConfigContent)
		assert.Contains(t, configContent, "capabilities:")
		assert.Contains(t, configContent, "kind: fs")
		assert.Contains(t, configContent, "kind: network")
		assert.Equal(t, "~/.reglet/config.yaml", result.ConfigPath)
	})

	t.Run("invalid options", func(t *testing.T) {
		opts := &InitOptions{
			ProfileName: "",
			Plugins:     []string{"file"},
			OutputPath:  "./test.yaml",
		}

		_, err := gen.Generate(opts)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid options")
	})
}

func TestGetPluginExample(t *testing.T) {
	t.Run("valid plugin", func(t *testing.T) {
		example := GetPluginExample("file")
		require.NotNil(t, example)
		assert.Equal(t, "file", example.PluginName)
		assert.Equal(t, "file-exists-check", example.ControlID)
	})

	t.Run("invalid plugin", func(t *testing.T) {
		example := GetPluginExample("nonexistent")
		assert.Nil(t, example)
	})

	t.Run("all plugins have examples", func(t *testing.T) {
		for _, plugin := range AvailablePlugins {
			example := GetPluginExample(plugin.Name)
			require.NotNil(t, example, "missing example for plugin %s", plugin.Name)
			assert.NotEmpty(t, example.ControlID)
			assert.NotEmpty(t, example.ExpectExpressions)
			assert.NotEmpty(t, example.Capabilities)
		}
	})
}

func TestGetCapabilitiesForPlugins(t *testing.T) {
	t.Run("single plugin", func(t *testing.T) {
		caps := GetCapabilitiesForPlugins([]string{"file"})
		require.Len(t, caps, 1)
		assert.Equal(t, "fs", caps[0].Kind)
	})

	t.Run("multiple plugins", func(t *testing.T) {
		caps := GetCapabilitiesForPlugins([]string{"file", "http"})
		require.Len(t, caps, 2)
	})

	t.Run("deduplication", func(t *testing.T) {
		// Two calls with same plugins should return same capabilities
		caps1 := GetCapabilitiesForPlugins([]string{"file", "http"})
		caps2 := GetCapabilitiesForPlugins([]string{"file", "http"})
		assert.Len(t, caps1, len(caps2))
	})

	t.Run("unknown plugin ignored", func(t *testing.T) {
		caps := GetCapabilitiesForPlugins([]string{"file", "unknown"})
		require.Len(t, caps, 1)
		assert.Equal(t, "fs", caps[0].Kind)
	})
}

func TestValidPluginNames(t *testing.T) {
	names := ValidPluginNames()

	assert.True(t, names["file"])
	assert.True(t, names["http"])
	assert.True(t, names["dns"])
	assert.True(t, names["tcp"])
	assert.True(t, names["command"])
	assert.True(t, names["smtp"])
	assert.False(t, names["invalid"])
	assert.False(t, names[""])
}
