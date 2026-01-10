package secrets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/reglet-dev/reglet/internal/infrastructure/sensitivedata"
	"github.com/reglet-dev/reglet/internal/infrastructure/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolver_Resolve(t *testing.T) {
	// Create a temp file for file-based secret
	tempDir := t.TempDir()
	secretFile := filepath.Join(tempDir, "mysecret.txt")
	err := os.WriteFile(secretFile, []byte("  file-secret-value  "), 0o600) // with whitespace to test trim
	require.NoError(t, err)

	provider := sensitivedata.NewProvider()
	config := &system.SecretsConfig{
		Local: map[string]string{
			"local_key": "local_value",
		},
		Env: map[string]string{
			"env_key": "TEST_ENV_SECRET",
		},
		Files: map[string]string{
			"file_key": secretFile,
		},
	}

	// Set env var
	t.Setenv("TEST_ENV_SECRET", "env_value")

	resolver := NewResolver(config, provider)

	tests := []struct {
		name          string
		secretName    string
		wantValue     string
		wantErr       bool
		wantInTracker bool
	}{
		{
			name:          "Local secret",
			secretName:    "local_key",
			wantValue:     "local_value",
			wantInTracker: true,
		},
		{
			name:          "Env secret",
			secretName:    "env_key",
			wantValue:     "env_value",
			wantInTracker: true,
		},
		{
			name:          "File secret",
			secretName:    "file_key",
			wantValue:     "file-secret-value", // trimmed
			wantInTracker: true,
		},
		{
			name:          "Unknown secret",
			secretName:    "unknown",
			wantErr:       true,
			wantInTracker: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := resolver.Resolve(tt.secretName)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantValue, val)

			if tt.wantInTracker {
				assert.Contains(t, provider.AllValues(), tt.wantValue)
			}
		})
	}
}

func TestResolver_Caching(t *testing.T) {
	provider := sensitivedata.NewProvider()
	config := &system.SecretsConfig{
		Local: map[string]string{
			"key": "value1",
		},
	}
	resolver := NewResolver(config, provider)

	// First call
	val, err := resolver.Resolve("key")
	require.NoError(t, err)
	assert.Equal(t, "value1", val)

	// Modify config backing source (hack to test cache)
	config.Local["key"] = "value2"

	// Second call should return cached value
	val, err = resolver.Resolve("key")
	require.NoError(t, err)
	assert.Equal(t, "value1", val)
}
