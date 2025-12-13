package system

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigLoader_Load_FileNotExists(t *testing.T) {
	loader := NewConfigLoader()
	cfg, err := loader.Load("/nonexistent/config.yaml")

	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Empty(t, cfg.Capabilities)
}

func TestConfigLoader_Load_ValidConfig(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yaml := `
capabilities:
  - kind: fs:read
    pattern: /etc/hosts
  - kind: network:outbound
    pattern: "*.example.com:443"

redaction:
  patterns:
    - "password\\s*=\\s*\\S+"
  paths:
    - "config.password"
    - "config.api_key"
  hash_mode:
    enabled: true
    salt: "test-salt"
`
	err := os.WriteFile(configPath, []byte(yaml), 0600)
	require.NoError(t, err)

	loader := NewConfigLoader()
	cfg, err := loader.Load(configPath)

	require.NoError(t, err)
	assert.Len(t, cfg.Capabilities, 2)
	assert.Equal(t, "fs:read", cfg.Capabilities[0].Kind)
	assert.Equal(t, "/etc/hosts", cfg.Capabilities[0].Pattern)

	assert.Len(t, cfg.Redaction.Patterns, 1)
	assert.Len(t, cfg.Redaction.Paths, 2)
	assert.True(t, cfg.Redaction.HashMode.Enabled)
	assert.Equal(t, "test-salt", cfg.Redaction.HashMode.Salt)
}

func TestConfig_ToHostFuncsCapabilities(t *testing.T) {
	cfg := &Config{
		Capabilities: []struct {
			Kind    string `yaml:"kind"`
			Pattern string `yaml:"pattern"`
		}{
			{Kind: "fs:read", Pattern: "/etc/hosts"},
			{Kind: "network:outbound", Pattern: "*.example.com:443"},
		},
	}

	caps := cfg.ToHostFuncsCapabilities()

	require.Len(t, caps, 2)
	assert.Equal(t, "fs:read", caps[0].Kind)
	assert.Equal(t, "/etc/hosts", caps[0].Pattern)
	assert.Equal(t, "network:outbound", caps[1].Kind)
	assert.Equal(t, "*.example.com:443", caps[1].Pattern)
}
