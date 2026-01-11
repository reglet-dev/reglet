package repository

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/values"
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
