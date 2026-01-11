package repository

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/values"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFSPluginRepository(t *testing.T) {
	// Create temp dir for tests
	tmpDir, err := os.MkdirTemp("", "reglet-plugins-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repo, err := NewFSPluginRepository(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create repo: %v", err)
	}

	ref := values.NewPluginReference("reg", "org", "repo", "name", "1.0")
	digest, _ := values.NewDigest("sha256", "abc")
	meta := values.NewPluginMetadata("name", "1.0", "desc", []string{"net"})
	plugin := entities.NewPlugin(ref, digest, meta)
	wasmContent := []byte("fake wasm content")

	t.Run("Store", func(t *testing.T) {
		wasmReader := bytes.NewReader(wasmContent)
		path, err := repo.Store(context.Background(), plugin, wasmReader)
		if err != nil {
			t.Fatalf("Store failed: %v", err)
		}

		if _, err := os.Stat(path); err != nil {
			t.Error("WASM file not created")
		}

		// Check metadata file
		metaPath := filepath.Join(filepath.Dir(path), "metadata.json")
		if _, err := os.Stat(metaPath); err != nil {
			t.Error("Metadata file not created")
		}

		// Check digest file
		digestPath := filepath.Join(filepath.Dir(path), "digest.txt")
		if _, err := os.Stat(digestPath); err != nil {
			t.Error("Digest file not created")
		}
	})

	t.Run("Find", func(t *testing.T) {
		got, path, err := repo.Find(context.Background(), ref)
		if err != nil {
			t.Fatalf("Find failed: %v", err)
		}

		if !got.Reference().Equals(ref) {
			t.Error("Found plugin has wrong reference")
		}
		if got.Digest().Value() != digest.Value() {
			t.Error("Found plugin has wrong digest")
		}
		if _, err := os.Stat(path); err != nil {
			t.Error("Returned path does not exist")
		}
	})

	t.Run("Find_NotFound", func(t *testing.T) {
		badRef := values.NewPluginReference("reg", "org", "repo", "missing", "1.0")
		_, _, err := repo.Find(context.Background(), badRef)
		if err == nil {
			t.Error("Find should fail for missing plugin")
		}
	})

	t.Run("List", func(t *testing.T) {
		list, err := repo.List(context.Background())
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		if len(list) != 1 {
			t.Errorf("Expected 1 plugin, got %d", len(list))
			return
		}
		if !list[0].Reference().Equals(ref) {
			t.Error("Listed plugin does not match")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		if err := repo.Delete(context.Background(), ref); err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		_, _, err := repo.Find(context.Background(), ref)
		if err == nil {
			t.Error("Find should fail after delete")
		}
	})
}

// TestFSPluginRepository_PathTraversalSecurity verifies protection against path traversal attacks.
func TestFSPluginRepository_PathTraversalSecurity(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "reglet-security-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	repo, err := NewFSPluginRepository(tmpDir)
	require.NoError(t, err)

	tests := []struct {
		name        string
		ref         values.PluginReference
		expectError bool
		errorMsg    string
	}{
		{
			name:        "PathTraversal_ParentDirectory",
			ref:         values.NewPluginReference("", "", "", "../../../etc/passwd", "1.0.0"),
			expectError: true,
			errorMsg:    "security violation",
		},
		{
			name:        "PathTraversal_AbsolutePath",
			ref:         values.NewPluginReference("/etc", "passwd", "repo", "file", "1.0.0"),
			expectError: true,
			errorMsg:    "security violation",
		},
		{
			name:        "PathTraversal_MixedDots",
			ref:         values.NewPluginReference("reg", "..", "..", "passwd", "1.0.0"),
			expectError: true,
			errorMsg:    "security violation",
		},
		{
			name:        "ValidPath_NoTraversal",
			ref:         values.NewPluginReference("reg.io", "org", "repo", "valid-plugin", "1.0.0"),
			expectError: false,
		},
		{
			name:        "ValidPath_Embedded",
			ref:         values.NewPluginReference("", "", "", "simple-plugin", ""),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test via pluginPath (internal method)
			path, err := repo.pluginPath(tt.ref)

			if tt.expectError {
				require.Error(t, err, "Expected error for malicious path")
				if tt.errorMsg != "" {
					assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.errorMsg),
						"Error message should mention security violation")
				}
				assert.Empty(t, path, "Path should be empty on error")
			} else {
				require.NoError(t, err, "Valid paths should not error")
				assert.NotEmpty(t, path, "Valid path should be returned")
				// Verify path is within tmpDir
				assert.True(t, strings.HasPrefix(path, tmpDir),
					"Valid path should be within repository root")
			}
		})
	}
}

// TestFSPluginRepository_Find_PathTraversal verifies Store/Find reject malicious paths.
func TestFSPluginRepository_Find_PathTraversal(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "reglet-security-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	repo, err := NewFSPluginRepository(tmpDir)
	require.NoError(t, err)

	// Attempt to store plugin with traversal in reference
	maliciousRef := values.NewPluginReference("", "", "", "../../malicious", "1.0.0")
	digest, _ := values.NewDigest("sha256", "abc123")
	meta := values.NewPluginMetadata("malicious", "1.0.0", "bad", []string{})
	plugin := entities.NewPlugin(maliciousRef, digest, meta)

	wasmContent := []byte("fake wasm")
	_, err = repo.Store(context.Background(), plugin, bytes.NewReader(wasmContent))

	// Should reject the malicious path
	require.Error(t, err, "Store should reject path traversal")
	assert.Contains(t, strings.ToLower(err.Error()), "security violation",
		"Error should indicate security violation detection")

	// Find should also reject
	_, _, err = repo.Find(context.Background(), maliciousRef)
	require.Error(t, err, "Find should reject path traversal")
}
