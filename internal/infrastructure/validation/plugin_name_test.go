package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
)

// Test_validatePluginName_ValidNames tests that valid plugin names are accepted
func Test_validatePluginName_ValidNames(t *testing.T) {
	validNames := []string{
		"file",
		"http",
		"my-plugin",
		"test_plugin2",
		"AWS",
		"plugin-123",
		"test_2024",
	}

	for _, name := range validNames {
		t.Run(name, func(t *testing.T) {
			err := ValidatePluginName(name)
			assert.NoError(t, err, "valid plugin name should not produce error")
		})
	}
}

// Test_validatePluginName_PathTraversal tests that path traversal attempts are rejected
func Test_validatePluginName_PathTraversal(t *testing.T) {
	tests := []struct {
		name        string
		pluginName  string
		expectedErr string
	}{
		{
			name:        "parent directory",
			pluginName:  "../file",
			expectedErr: "plugin name cannot contain path separators", // Caught by "/" check first
		},
		{
			name:        "absolute path unix",
			pluginName:  "/etc/passwd",
			expectedErr: "plugin name cannot contain path separators",
		},
		{
			name:        "absolute path windows",
			pluginName:  "C:\\Windows\\System32",
			expectedErr: "plugin name cannot contain path separators",
		},
		{
			name:        "subdirectory",
			pluginName:  "plugins/malicious",
			expectedErr: "plugin name cannot contain path separators",
		},
		{
			name:        "just parent reference",
			pluginName:  "..",
			expectedErr: "plugin name cannot contain parent directory references",
		},
		{
			name:        "hidden traversal",
			pluginName:  "foo..bar",
			expectedErr: "plugin name cannot contain parent directory references",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePluginName(tt.pluginName)
			assert.Error(t, err, "path traversal attempt should be rejected")
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

// Test_validatePluginName_InvalidCharacters tests that invalid characters are rejected
func Test_validatePluginName_InvalidCharacters(t *testing.T) {
	tests := []struct {
		name       string
		pluginName string
	}{
		{
			name:       "spaces",
			pluginName: "my plugin",
		},
		{
			name:       "special chars",
			pluginName: "plugin@version",
		},
		{
			name:       "dots",
			pluginName: "plugin.wasm",
		},
		{
			name:       "semicolon",
			pluginName: "plugin;rm",
		},
		{
			name:       "ampersand",
			pluginName: "plugin&command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePluginName(tt.pluginName)
			assert.Error(t, err, "invalid characters should be rejected")
			assert.Contains(t, err.Error(), "must contain only alphanumeric characters, underscores, and hyphens")
		})
	}
}

// Test_validatePluginName_EdgeCases tests edge cases
func Test_validatePluginName_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		pluginName  string
		expectedErr string
	}{
		{
			name:        "empty string",
			pluginName:  "",
			expectedErr: "plugin name cannot be empty",
		},
		{
			name:        "too long",
			pluginName:  "this-is-a-very-long-plugin-name-that-exceeds-the-maximum-length-allowed-by-validation",
			expectedErr: "plugin name too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePluginName(tt.pluginName)
			assert.Error(t, err, "edge case should be rejected")
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

// Test_validateObservation_WithInvalidPluginName tests that observation validation catches invalid plugin names
func Test_validateObservation_WithInvalidPluginName(t *testing.T) {
	tests := []struct {
		name        string
		obs         entities.Observation
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid plugin name",
			obs: entities.Observation{
				Plugin: "file",
				Config: map[string]interface{}{},
			},
			expectError: false,
		},
		{
			name: "path traversal in plugin name",
			obs: entities.Observation{
				Plugin: "../etc/passwd",
				Config: map[string]interface{}{},
			},
			expectError: true,
			errorMsg:    "invalid plugin name",
		},
		{
			name: "empty plugin name",
			obs: entities.Observation{
				Plugin: "",
				Config: map[string]interface{}{},
			},
			expectError: true,
			errorMsg:    "plugin name is required",
		},
		{
			name: "plugin name with slash",
			obs: entities.Observation{
				Plugin: "plugins/file",
				Config: map[string]interface{}{},
			},
			expectError: true,
			errorMsg:    "invalid plugin name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateObservation(tt.obs)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
