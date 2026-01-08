package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_validatePluginName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid simple name",
			input:   "mycheck",
			wantErr: false,
		},
		{
			name:    "valid with hyphen",
			input:   "my-check",
			wantErr: false,
		},
		{
			name:    "valid with numbers",
			input:   "check2go",
			wantErr: false,
		},
		{
			name:    "valid single letter",
			input:   "a",
			wantErr: false,
		},
		{
			name:    "empty name",
			input:   "",
			wantErr: true,
			errMsg:  "plugin name is required",
		},
		{
			name:    "starts with number",
			input:   "2check",
			wantErr: true,
			errMsg:  "must be lowercase alphanumeric",
		},
		{
			name:    "uppercase letters",
			input:   "MyCheck",
			wantErr: true,
			errMsg:  "must be lowercase",
		},
		{
			name:    "ends with hyphen",
			input:   "check-",
			wantErr: true,
			errMsg:  "must be lowercase alphanumeric",
		},
		{
			name:    "consecutive hyphens",
			input:   "my--check",
			wantErr: true,
			errMsg:  "consecutive hyphens not allowed",
		},
		{
			name:    "special characters",
			input:   "my_check",
			wantErr: true,
			errMsg:  "must be lowercase alphanumeric",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validatePluginName(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_toTitleCase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"my-plugin", "My Plugin"},
		{"simple", "Simple"},
		{"dns-check", "Dns Check"},
		{"a-b-c", "A B C"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, toTitleCase(tt.input))
		})
	}
}

func Test_toPluginStructName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"my-plugin", "myPluginPlugin"},
		{"simple", "simplePlugin"},
		{"dns-check", "dnsCheckPlugin"},
		{"a-b-c", "aBCPlugin"},
		{"file", "filePlugin"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, toPluginStructName(tt.input))
		})
	}
}

func Test_parseCapabilities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   []string
		wantLen int
		wantErr bool
	}{
		{
			name:    "empty",
			input:   nil,
			wantLen: 0,
			wantErr: false,
		},
		{
			name:    "single with pattern",
			input:   []string{"network:dns"},
			wantLen: 1,
			wantErr: false,
		},
		{
			name:    "single without pattern",
			input:   []string{"fs"},
			wantLen: 1,
			wantErr: false,
		},
		{
			name:    "multiple",
			input:   []string{"network:dns", "fs:read"},
			wantLen: 2,
			wantErr: false,
		},
		{
			name:    "complex pattern",
			input:   []string{"network:tcp:443"},
			wantLen: 1,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			caps, err := parseCapabilities(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, caps, tt.wantLen)
			}
		})
	}
}

func Test_runCreatePlugin_Success(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "test-plugin")

	opts := &CreatePluginOptions{
		name:         "test-plugin",
		lang:         "go",
		output:       outputDir,
		modulePath:   "github.com/test/test-plugin",
		sdkVersion:   "v0.1.0",
		capabilities: []string{"network:dns", "fs:read"},
		force:        false,
	}

	err := runCreatePlugin(opts)
	require.NoError(t, err)

	// Verify files were created
	expectedFiles := []string{
		"plugin.go",
		"main.go",
		"go.mod",
		"Makefile",
		"plugin_test.go",
		"README.md",
	}

	for _, file := range expectedFiles {
		path := filepath.Join(outputDir, file)
		_, err := os.Stat(path)
		assert.NoError(t, err, "expected file to exist: %s", file)

		// Verify file is not empty
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.NotEmpty(t, content, "file should not be empty: %s", file)
	}

	// Verify plugin.go contains expected content
	pluginContent, err := os.ReadFile(filepath.Join(outputDir, "plugin.go"))
	require.NoError(t, err)
	assert.Contains(t, string(pluginContent), "testPluginPlugin")
	assert.Contains(t, string(pluginContent), `Name:        "test-plugin"`)
}

func Test_runCreatePlugin_ExistingFile(t *testing.T) {
	// Create temp directory with existing file
	tmpDir := t.TempDir()
	pluginFile := filepath.Join(tmpDir, "plugin.go")
	require.NoError(t, os.WriteFile(pluginFile, []byte("existing"), 0o644))

	opts := &CreatePluginOptions{
		name:   "test",
		lang:   "go",
		output: tmpDir,
		force:  false,
	}

	err := runCreatePlugin(opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file already exists")
}

func Test_runCreatePlugin_ForceOverwrite(t *testing.T) {
	// Create temp directory with existing file
	tmpDir := t.TempDir()
	pluginFile := filepath.Join(tmpDir, "plugin.go")
	require.NoError(t, os.WriteFile(pluginFile, []byte("existing"), 0o644))

	opts := &CreatePluginOptions{
		name:       "test",
		lang:       "go",
		output:     tmpDir,
		modulePath: "github.com/test/test",
		sdkVersion: "v0.1.0",
		force:      true,
	}

	err := runCreatePlugin(opts)
	require.NoError(t, err)

	// Verify file was overwritten
	content, err := os.ReadFile(pluginFile)
	require.NoError(t, err)
	assert.NotEqual(t, "existing", string(content))
}

func Test_runCreatePlugin_UnsupportedLanguage(t *testing.T) {
	opts := &CreatePluginOptions{
		name: "test",
		lang: "rust",
	}

	err := runCreatePlugin(opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported language")
}
