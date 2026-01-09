package capabilities

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/reglet-dev/reglet/internal/domain/capabilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileStore_LoadAndSave(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for config files
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	store := NewFileStore(configPath)

	// Test loading from non-existent file (should return empty grant)
	grants, err := store.Load()
	require.NoError(t, err)
	assert.Empty(t, grants)

	// Create some grants
	grant1 := capabilities.Capability{Kind: "fs", Pattern: "read:/etc/passwd"}
	grant2 := capabilities.Capability{Kind: "network", Pattern: "outbound:80"}
	testGrants := capabilities.NewGrant()
	testGrants.Add(grant1)
	testGrants.Add(grant2)

	// Save grants
	err = store.Save(testGrants)
	require.NoError(t, err)

	// Verify file content
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	expectedContent := `capabilities:
  - kind: fs
    pattern: read:/etc/passwd
  - kind: network
    pattern: outbound:80
`
	assert.Equal(t, expectedContent, string(content))

	// Load grants back
	loadedGrants, err := store.Load()
	require.NoError(t, err)
	assert.Len(t, loadedGrants, 2)
	assert.True(t, loadedGrants.Contains(grant1))
	assert.True(t, loadedGrants.Contains(grant2))
}

func TestFileStore_Load_InvalidYAML(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	store := NewFileStore(configPath)

	// Write invalid YAML to file
	err := os.WriteFile(configPath, []byte("invalid yaml: ---\n-"), 0o600)
	require.NoError(t, err)

	_, err = store.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse config file")
}

func TestFileStore_Save_DirectoryCreation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "nested", "config.yaml")
	store := NewFileStore(nestedPath)

	err := store.Save(capabilities.NewGrant())
	require.NoError(t, err)

	// Verify directory was created
	_, err = os.Stat(filepath.Dir(nestedPath))
	assert.False(t, os.IsNotExist(err))
}

func TestFileStore_Load_EmptyCapabilities(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	store := NewFileStore(configPath)

	err := os.WriteFile(configPath, []byte("capabilities: []\n"), 0o600)
	require.NoError(t, err)

	grants, err := store.Load()
	require.NoError(t, err)
	assert.Empty(t, grants)
}

func TestFileStore_Save_EmptyCapabilities(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	store := NewFileStore(configPath)

	err := store.Save(capabilities.NewGrant())
	require.NoError(t, err)

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var cfg configFile // Use the infrastructure configFile struct
	err = yaml.Unmarshal(content, &cfg)
	require.NoError(t, err)
	assert.Empty(t, cfg.Capabilities, "Expected no capabilities in saved config for empty grant")
}
