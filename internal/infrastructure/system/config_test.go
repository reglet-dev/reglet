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
	assert.True(t, cfg.GrantSet.IsEmpty())
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
	assert.True(t, cfg.GrantSet.IsEmpty())
	assert.Equal(t, string(SecurityLevelStandard), cfg.Security.Level)

	// Verify maps are initialized (can be used immediately)
	assert.NotNil(t, cfg.SensitiveData.Secrets.Local)
}

func TestConfigLoader_Load_ValidConfig(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yaml := `
fs:
  rules:
    - read:
        - /etc/hosts
network:
  rules:
    - hosts:
        - "*.example.com"
      ports:
        - "443"

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
	err := os.WriteFile(configPath, []byte(yaml), 0o644)
	require.NoError(t, err)

	loader := NewConfigLoader()
	cfg, err := loader.Load(configPath)

	require.NoError(t, err)
	assert.NotNil(t, cfg.FS)
	assert.Len(t, cfg.FS.Rules, 1)
	assert.Equal(t, "/etc/hosts", cfg.FS.Rules[0].Read[0])

	assert.NotNil(t, cfg.Network)
	assert.Len(t, cfg.Network.Rules, 1)
	assert.Equal(t, "*.example.com", cfg.Network.Rules[0].Hosts[0])
	assert.Equal(t, "443", cfg.Network.Rules[0].Ports[0])

	assert.Len(t, cfg.Redaction.Patterns, 1)
	assert.Len(t, cfg.Redaction.Paths, 2)
	assert.True(t, cfg.Redaction.HashMode.Enabled)
	assert.Equal(t, "test-salt", cfg.Redaction.HashMode.Salt)
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
	err := os.WriteFile(configPath, []byte(yaml), 0o644)
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
