package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedaction_EndToEnd(t *testing.T) {
	// 1. Setup Paths
	rootDir := findProjectRoot(t)
	binPath := filepath.Join(rootDir, "bin", "reglet")

	// Ensure binary is built
	cmd := exec.Command("make", "build")
	cmd.Dir = rootDir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to build reglet: %s", out)

	// 2. Create a temporary home directory for config
	tempHome := t.TempDir()
	configDir := filepath.Join(tempHome, ".reglet")
	err = os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	// 3. Create config.yaml with redaction rules
	configContent := `
redaction:
  patterns:
    - "SECRET-[A-Z0-9]{8}"
  paths:
    - "api_key"
  hash_mode:
    enabled: false
`
	err = os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configContent), 0600)
	require.NoError(t, err)

	// 4. Create a test profile
	profilePath := filepath.Join(tempHome, "redaction-test.yaml")
	profileContent := `
profile:
  name: redaction-test
  version: 1.0.0

controls:
  items:
    - id: secret-echo
      name: Secret Echo Check
      observations:
        - plugin: command
          config:
            run: echo "My secret is SECRET-ABC12345"
            env:
              - "MY_API_KEY=top-secret-value"
          expect:
            - data.exit_code == 0
`
	err = os.WriteFile(profilePath, []byte(profileContent), 0600)
	require.NoError(t, err)

	// 5. Run reglet check
	// We need to set HOME to point to our temp dir so it picks up the config
	checkCmd := exec.Command(binPath, "check", profilePath, "--format", "json", "--trust-plugins")
	checkCmd.Env = append(os.Environ(), "HOME="+tempHome)

	outputBytes, err := checkCmd.CombinedOutput()
	require.NoError(t, err, "Reglet check failed: %s", outputBytes)

	// Parse JSON output (skip log lines)
	outputStr := string(outputBytes)
	jsonStart := 0
	for i, char := range outputStr {
		if char == '{' {
			jsonStart = i
			break
		}
	}
	jsonStr := outputStr[jsonStart:]

	var result map[string]interface{}
	err = json.Unmarshal([]byte(jsonStr), &result)
	require.NoError(t, err, "Failed to parse JSON output")

	// 6. Verify Redaction
	// Navigate to controls[0].observations[0].evidence.Data.stdout
	controls := result["controls"].([]interface{})
	obs := controls[0].(map[string]interface{})["observations"].([]interface{})
	evidence := obs[0].(map[string]interface{})["evidence"].(map[string]interface{})
	data := evidence["Data"].(map[string]interface{})

	stdout := data["stdout"].(string)
	assert.Contains(t, stdout, "My secret is [REDACTED]")
	assert.NotContains(t, stdout, "SECRET-ABC12345")
}

func findProjectRoot(t *testing.T) string {
	wd, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("could not find project root")
		}
		wd = parent
	}
}
