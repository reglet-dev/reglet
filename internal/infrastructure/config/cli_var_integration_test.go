// Package config provides configuration parsing and variable handling.
// This file contains integration tests for CLI variable overrides.
package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_BasicSetOverride tests basic --set override end-to-end.
// Covers T010: Create integration test for basic --set override.
func TestIntegration_BasicSetOverride(t *testing.T) {
	// Simulate profile with vars
	profileContent := `
profile:
  name: test-profile
  version: "1.0"

vars:
  environment: dev
  region: us-east-1

controls:
  items:
    - id: env-check
      name: Environment Check
      observe:
        - plugin: command
          config:
            command: echo "{{ .vars.environment }}"
`
	// Parse CLI vars that override profile vars
	cliVars, err := ParseMultipleCLIVars([]string{"environment=prod"})
	require.NoError(t, err)

	// Merge with profile vars
	profileVars := map[string]interface{}{
		"environment": "dev",
		"region":      "us-east-1",
	}
	merged := MergeCLIVars(profileVars, cliVars)

	// Verify override happened
	assert.Equal(t, "prod", merged["environment"])
	assert.Equal(t, "us-east-1", merged["region"])

	// Verify unused var detection doesn't flag used var
	unused := FindUnusedVars(cliVars, profileContent)
	assert.Empty(t, unused, "environment var should be detected as used")
}

// TestIntegration_NestedSetOverride tests --set with nested paths.
// Covers T011: Create integration test for nested --set override (paths.config).
func TestIntegration_NestedSetOverride(t *testing.T) {
	profileContent := `
vars:
  paths:
    config: /etc/default
    data: /var/data

controls:
  items:
    - id: config-check
      observe:
        - plugin: file
          config:
            path: "{{ .vars.paths.config }}/app.conf"
`
	// Parse nested CLI var
	cliVars, err := ParseMultipleCLIVars([]string{"paths.config=/opt/custom"})
	require.NoError(t, err)

	// Merge with profile vars (deep nested structure)
	profileVars := map[string]interface{}{
		"paths": map[string]interface{}{
			"config": "/etc/default",
			"data":   "/var/data",
		},
	}
	merged := MergeCLIVars(profileVars, cliVars)

	// Verify nested override
	paths := merged["paths"].(map[string]interface{})
	assert.Equal(t, "/opt/custom", paths["config"])
	assert.Equal(t, "/var/data", paths["data"]) // Preserved

	// Verify unused var detection for nested vars
	unused := FindUnusedVars(cliVars, profileContent)
	assert.Empty(t, unused, "paths var should be detected as used")
}

// TestIntegration_MultipleSetFlags tests multiple --set flags with last-wins semantics.
// Covers T012: Create integration test for multiple --set flags.
func TestIntegration_MultipleSetFlags(t *testing.T) {
	// Simulate multiple --set flags passed by user
	cliInputs := []string{
		"env=dev",
		"port=8080",
		"debug=true",
		"env=prod", // Later value wins for same key
	}

	cliVars, err := ParseMultipleCLIVars(cliInputs)
	require.NoError(t, err)

	// Verify last-wins semantics
	assert.Equal(t, "prod", cliVars["env"], "last value should win for duplicate key")
	assert.Equal(t, int64(8080), cliVars["port"])
	assert.Equal(t, true, cliVars["debug"])
}

// TestIntegration_InjectNewVariable tests injecting a variable not defined in profile.
// Covers T019: Create test for injecting new variable not in profile.
func TestIntegration_InjectNewVariable(t *testing.T) {
	// Profile content that uses dynamic variables
	profileContent := `
vars:
  environment: dev

controls:
  items:
    - id: build-info
      observe:
        - plugin: command
          config:
            command: echo "Build: {{ .vars.build_id }}"
`
	// CLI injects a new variable not in profile vars
	cliVars, err := ParseMultipleCLIVars([]string{"build_id=abc123"})
	require.NoError(t, err)

	// Merge - new var should be added
	profileVars := map[string]interface{}{
		"environment": "dev",
	}
	merged := MergeCLIVars(profileVars, cliVars)

	// Verify new var is present
	assert.Equal(t, "abc123", merged["build_id"])
	assert.Equal(t, "dev", merged["environment"]) // Preserved

	// Verify it's detected as used in profile
	unused := FindUnusedVars(cliVars, profileContent)
	assert.Empty(t, unused, "build_id should be detected as used")
}

// TestIntegration_UnusedVariableWarning tests detection of unused CLI vars.
// Covers T020: Create test for unused variable warning detection.
func TestIntegration_UnusedVariableWarning(t *testing.T) {
	profileContent := `
vars:
  environment: dev

controls:
  items:
    - id: check
      observe:
        - plugin: command
          config:
            command: echo "{{ .vars.environment }}"
`
	// CLI sets vars that aren't used in profile
	cliVars, err := ParseMultipleCLIVars([]string{
		"environment=prod", // Used
		"typo_var=value",   // Not used - typo
		"unused_key=value", // Not used
	})
	require.NoError(t, err)

	unused := FindUnusedVars(cliVars, profileContent)

	// Should detect unused vars
	assert.Len(t, unused, 2)
	assert.Contains(t, unused, "typo_var")
	assert.Contains(t, unused, "unused_key")
}

// TestIntegration_SetFileAndSetEnv tests --set-file and --set-env together.
func TestIntegration_SetFileAndSetEnv(t *testing.T) {
	// Create temp file for --set-file
	tmpDir := t.TempDir()
	secretFile := filepath.Join(tmpDir, "api_key.txt")
	err := os.WriteFile(secretFile, []byte("secret-api-key-123\n"), 0o600)
	require.NoError(t, err)

	// Set env var for --set-env
	t.Setenv("CLUSTER_NAME", "production-cluster")

	// Parse --set-file
	key1, val1, err := ParseSetFile("api_key=" + secretFile)
	require.NoError(t, err)
	assert.Equal(t, "api_key", key1)
	assert.Equal(t, "secret-api-key-123", val1) // Trailing newline trimmed

	// Parse --set-env
	key2, val2, err := ParseSetEnv("cluster=CLUSTER_NAME")
	require.NoError(t, err)
	assert.Equal(t, "cluster", key2)
	assert.Equal(t, "production-cluster", val2)

	// Build merged vars as check command would
	cliVars := make(map[string]interface{})
	err = SetNestedValue(cliVars, key1, val1)
	require.NoError(t, err)
	err = SetNestedValue(cliVars, key2, val2)
	require.NoError(t, err)

	assert.Equal(t, "secret-api-key-123", cliVars["api_key"])
	assert.Equal(t, "production-cluster", cliVars["cluster"])
}

// TestIntegration_TypeDetectionInContext tests type detection in a realistic context.
func TestIntegration_TypeDetectionInContext(t *testing.T) {
	// Simulate --set flags with various types
	cliInputs := []string{
		"max_connections=100",      // Integer
		"timeout=30.5",             // Float
		"debug=true",               // Boolean
		"version=1.0.0",            // String (version, not float)
		"port=08080",               // String (leading zero)
		"ssl_enabled=false",        // Boolean
		"api_url=https://api.test", // String
	}

	cliVars, err := ParseMultipleCLIVars(cliInputs)
	require.NoError(t, err)

	// Verify types are correctly detected
	assert.Equal(t, int64(100), cliVars["max_connections"])
	assert.Equal(t, 30.5, cliVars["timeout"])
	assert.Equal(t, true, cliVars["debug"])
	assert.Equal(t, "1.0.0", cliVars["version"]) // Preserved as string
	assert.Equal(t, "08080", cliVars["port"])    // Preserved as string (leading zero)
	assert.Equal(t, false, cliVars["ssl_enabled"])
	assert.Equal(t, "https://api.test", cliVars["api_url"])
}

// TestIntegration_TemplateInjectionPrevention verifies CLI values are treated as literals.
// Covers T038: Create tests for template injection prevention.
func TestIntegration_TemplateInjectionPrevention(t *testing.T) {
	// Malicious input attempting template injection
	maliciousInputs := []string{
		`command={{ secret "admin_password" }}`,     // Attempt to read secret
		`query=DELETE FROM users; --`,               // SQL injection attempt
		`path={{ .vars.path }}/../../../etc/passwd`, // Path traversal attempt
		`script=<script>alert('xss')</script>`,      // XSS attempt
	}

	for _, input := range maliciousInputs {
		cliVars, err := ParseMultipleCLIVars([]string{input})
		require.NoError(t, err, "Parsing should succeed for: %s", input)

		// The value should be stored EXACTLY as provided, never re-interpreted
		for _, v := range cliVars {
			// Values are stored as literal strings, not re-parsed as templates
			assert.IsType(t, "", v, "Value should be a string literal")
		}
	}

	// Specific test: template syntax in value should be literal
	cliVars, err := ParseMultipleCLIVars([]string{`test={{ .vars.foo }}`})
	require.NoError(t, err)
	assert.Equal(t, "{{ .vars.foo }}", cliVars["test"], "Template syntax should be literal, not evaluated")
}
