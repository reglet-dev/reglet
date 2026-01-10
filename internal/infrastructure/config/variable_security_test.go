package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVariableSubstitution_TemplateInjection tests for template injection vulnerabilities.
// This verifies that variable values cannot inject template syntax that would be re-evaluated.
func TestVariableSubstitution_TemplateInjection(t *testing.T) {
	tests := []struct {
		name           string
		yaml           string
		expectedPath   string
		shouldContain  string
		shouldNotMatch string
		description    string
	}{
		{
			name: "template syntax in variable value",
			yaml: `
profile:
  name: injection-test
  version: 1.0.0

vars:
  # Malicious value attempting template injection
  evil_path: "{{ .vars.other_var }}"
  other_var: "/etc/passwd"

controls:
  items:
    - id: test
      name: Test Control
      observations:
        - plugin: file
          config:
            path: "{{ .vars.evil_path }}"
`,
			// Variable value should be treated as literal string, not re-evaluated
			expectedPath:   "{{ .vars.other_var }}",
			shouldNotMatch: "/etc/passwd",
			description:    "Template syntax in variable values should not be re-evaluated",
		},
		{
			name: "environment variable injection attempt",
			yaml: `
profile:
  name: env-injection-test
  version: 1.0.0

vars:
  # If template functions are implemented, this could leak environment variables
  evil: "{{ env \"AWS_SECRET_ACCESS_KEY\" }}"

controls:
  items:
    - id: test
      name: Test Control
      observations:
        - plugin: file
          config:
            path: "{{ .vars.evil }}"
`,
			expectedPath: "{{ env \"AWS_SECRET_ACCESS_KEY\" }}",
			description:  "env function calls in variable values should not be executed",
		},
		{
			name: "nested template injection",
			yaml: `
profile:
  name: nested-injection
  version: 1.0.0

vars:
  inner: "SECRET_VALUE"
  # Attempting to use index/nested access
  evil: "{{ index .vars \"inner\" }}"

controls:
  items:
    - id: test
      name: Test Control
      observations:
        - plugin: file
          config:
            path: "{{ .vars.evil }}"
`,
			expectedPath:   "{{ index .vars \"inner\" }}",
			shouldNotMatch: "SECRET_VALUE",
			description:    "Template function calls should not execute in variable values",
		},
		{
			name: "command injection via variable",
			yaml: `
profile:
  name: command-injection
  version: 1.0.0

vars:
  # Attempting shell command injection
  evil_cmd: "ls; rm -rf /"

controls:
  items:
    - id: test
      name: Test Control
      observations:
        - plugin: command
          config:
            command: "{{ .vars.evil_cmd }}"
`,
			expectedPath:  "ls; rm -rf /",
			shouldContain: ";",
			description:   "Shell metacharacters should be preserved as-is (plugin must validate)",
		},
		{
			name: "YAML metacharacters in variable",
			yaml: `
profile:
  name: yaml-injection
  version: 1.0.0

vars:
  # YAML special characters
  evil: "value: with: colons\nand: newlines"

controls:
  items:
    - id: test
      name: Test Control
      observations:
        - plugin: file
          config:
            path: "{{ .vars.evil }}"
`,
			expectedPath:  "value: with: colons\nand: newlines",
			shouldContain: "\n",
			description:   "YAML metacharacters are safe (YAML already parsed)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := NewProfileLoader()
			profile, err := loader.LoadProfileFromReader(strings.NewReader(tt.yaml))
			require.NoError(t, err, "Profile should load successfully")

			substitutor := NewVariableSubstitutor(nil)
			err = substitutor.Substitute(profile)
			require.NoError(t, err, "Substitution should succeed")

			// Get the substituted value (try both "path" and "command" keys)
			config := profile.Controls.Items[0].ObservationDefinitions[0].Config
			actualPath, ok := config["path"]
			if !ok {
				actualPath = config["command"]
			}

			if tt.expectedPath != "" {
				assert.Equal(t, tt.expectedPath, actualPath,
					"%s: Expected literal substitution (no re-evaluation)", tt.description)
			}

			if tt.shouldContain != "" {
				actualStr, ok := actualPath.(string)
				require.True(t, ok, "Config value should be a string")
				assert.Contains(t, actualStr, tt.shouldContain,
					"%s: Should preserve special characters", tt.description)
			}

			if tt.shouldNotMatch != "" {
				assert.NotEqual(t, tt.shouldNotMatch, actualPath,
					"%s: Should NOT evaluate nested templates", tt.description)
			}
		})
	}
}

// TestVariableSubstitution_PathTraversal tests for path traversal vulnerabilities.
func TestVariableSubstitution_PathTraversal(t *testing.T) {
	yaml := `
profile:
  name: path-traversal
  version: 1.0.0

vars:
  # Path traversal attempt
  evil_path: "../../../etc/passwd"

controls:
  items:
    - id: test
      name: Test Control
      observations:
        - plugin: file
          config:
            path: "/safe/dir/{{ .vars.evil_path }}"
`

	loader := NewProfileLoader()
	profile, err := loader.LoadProfileFromReader(strings.NewReader(yaml))
	require.NoError(t, err)

	substitutor := NewVariableSubstitutor(nil)
	err = substitutor.Substitute(profile)
	require.NoError(t, err)

	actualPath := profile.Controls.Items[0].ObservationDefinitions[0].Config["path"]

	// Path traversal sequences should be preserved (plugin/capability system must validate)
	assert.Equal(t, "/safe/dir/../../../etc/passwd", actualPath,
		"Path traversal should be preserved as-is - capability system enforces access control")
}

// TestVariableSubstitution_SpecialCharacters verifies handling of special characters.
func TestVariableSubstitution_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name     string
		varValue string
		expected string
	}{
		{
			name:     "newline characters",
			varValue: "line1\\nline2",
			expected: "line1\nline2", // YAML parses \n as actual newline
		},
		{
			name:     "null bytes (should be preserved)",
			varValue: "value\\x00null",
			expected: "value\x00null", // YAML parses \x00 as actual null byte
		},
		{
			name:     "unicode characters",
			varValue: "测试中文",
			expected: "测试中文",
		},
		{
			name:     "emoji",
			varValue: "test 🚀 emoji",
			expected: "test 🚀 emoji",
		},
		{
			name:     "regex metacharacters",
			varValue: ".*$^[](){}|+?",
			expected: ".*$^[](){}|+?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yaml := `
profile:
  name: special-chars
  version: 1.0.0

vars:
  test_var: "` + tt.varValue + `"

controls:
  items:
    - id: test
      name: Test Control
      observations:
        - plugin: file
          config:
            value: "{{ .vars.test_var }}"
`

			loader := NewProfileLoader()
			profile, err := loader.LoadProfileFromReader(strings.NewReader(yaml))
			require.NoError(t, err)

			substitutor := NewVariableSubstitutor(nil)
			err = substitutor.Substitute(profile)
			require.NoError(t, err)

			actual := profile.Controls.Items[0].ObservationDefinitions[0].Config["value"]
			assert.Equal(t, tt.expected, actual, "Special characters should be preserved")
		})
	}
}
