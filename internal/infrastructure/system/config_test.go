package system

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	require.NotNil(t, cfg)

	// Verify all fields have sensible defaults
	assert.Empty(t, cfg.Capabilities)
	assert.Empty(t, cfg.Redaction.Patterns)
	assert.Empty(t, cfg.Redaction.Paths)
	assert.False(t, cfg.Redaction.HashMode.Enabled)
	assert.Equal(t, string(SecurityLevelStandard), cfg.Security.Level)
	assert.Empty(t, cfg.Security.CustomBroadPatterns)
	assert.Equal(t, 0, cfg.WasmMemoryLimitMB)
	assert.Equal(t, 0, cfg.MaxEvidenceSizeBytes)

	// Verify maps are initialized (not nil)
	assert.NotNil(t, cfg.SensitiveData.Secrets.Local)
	assert.NotNil(t, cfg.SensitiveData.Secrets.Env)
	assert.NotNil(t, cfg.SensitiveData.Secrets.Files)
}

func TestConfigLoader_Load_FileNotExists(t *testing.T) {
	loader := NewConfigLoader()
	cfg, err := loader.Load("/nonexistent/config.yaml")

	require.NoError(t, err)
	assert.NotNil(t, cfg)

	// Verify it returns DefaultConfig()
	assert.Empty(t, cfg.Capabilities)
	assert.Equal(t, string(SecurityLevelStandard), cfg.Security.Level)

	// Verify maps are initialized (can be used immediately)
	assert.NotNil(t, cfg.SensitiveData.Secrets.Local)
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
	err := os.WriteFile(configPath, []byte(yaml), 0644)
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
		Capabilities: []CapabilityConfig{
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

func TestSecurityConfig_GetSecurityLevel(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected SecurityLevel
	}{
		{
			name:     "strict level",
			level:    "strict",
			expected: SecurityLevelStrict,
		},
		{
			name:     "standard level",
			level:    "standard",
			expected: SecurityLevelStandard,
		},
		{
			name:     "permissive level",
			level:    "permissive",
			expected: SecurityLevelPermissive,
		},
		{
			name:     "empty defaults to standard",
			level:    "",
			expected: SecurityLevelStandard,
		},
		{
			name:     "invalid defaults to standard",
			level:    "invalid",
			expected: SecurityLevelStandard,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &SecurityConfig{
				Level: tt.level,
			}
			assert.Equal(t, tt.expected, cfg.GetSecurityLevel())
		})
	}
}

func TestConfigLoader_Load_WithSecurityConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yaml := `
security:
  level: strict
  custom_broad_patterns:
    - "fs:write:/tmp/**"
    - "network:outbound:*"
`
	err := os.WriteFile(configPath, []byte(yaml), 0644)
	require.NoError(t, err)

	loader := NewConfigLoader()
	cfg, err := loader.Load(configPath)

	require.NoError(t, err)
	assert.Equal(t, "strict", cfg.Security.Level)
	assert.Equal(t, SecurityLevelStrict, cfg.Security.GetSecurityLevel())
	assert.Len(t, cfg.Security.CustomBroadPatterns, 2)
	assert.Contains(t, cfg.Security.CustomBroadPatterns, "fs:write:/tmp/**")
	assert.Contains(t, cfg.Security.CustomBroadPatterns, "network:outbound:*")
}
