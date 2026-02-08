package profiles_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	hostValues "github.com/reglet-dev/reglet-host-sdk/plugin/values"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/values"
	"github.com/reglet-dev/reglet/internal/infrastructure/profiles"
)

func Test_NewFSProfileCacheRepository(t *testing.T) {
	t.Run("creates with custom root", func(t *testing.T) {
		tmpDir := t.TempDir()

		repo, err := profiles.NewFSProfileCacheRepository(tmpDir)

		require.NoError(t, err)
		assert.NotNil(t, repo)
		assert.Equal(t, tmpDir, repo.Root)
	})

	t.Run("creates default directory when empty root", func(t *testing.T) {
		// Skip this test if we can't get home directory
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skip("cannot get home directory")
		}

		repo, err := profiles.NewFSProfileCacheRepository("")

		require.NoError(t, err)
		assert.NotNil(t, repo)
		assert.Equal(t, filepath.Join(home, ".reglet", "profiles"), repo.Root)
	})

	t.Run("creates root directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		cacheDir := filepath.Join(tmpDir, "nested", "cache")

		repo, err := profiles.NewFSProfileCacheRepository(cacheDir)

		require.NoError(t, err)
		assert.NotNil(t, repo)

		// Directory should exist
		info, err := os.Stat(cacheDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})
}

func Test_FSProfileCacheRepository_StoreAndFind(t *testing.T) {
	tmpDir := t.TempDir()
	repo, err := profiles.NewFSProfileCacheRepository(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	// Create a test profile reference
	ref, err := values.ParseProfileReference("https://example.com/profiles/test.yaml")
	require.NoError(t, err)

	// Create test content
	content := []byte("controls:\n  - id: test-1\n")
	contentHash, err := hostValues.ComputeDigestSHA256(strings.NewReader(string(content)))
	require.NoError(t, err)

	// Create cache entry
	entry, err := entities.NewProfileCacheEntry(
		ref,
		content,
		contentHash,
		time.Hour, // TTL
	)
	require.NoError(t, err)
	entry.SetETag("\"etag-123\"")

	t.Run("store succeeds", func(t *testing.T) {
		err := repo.Store(ctx, entry)
		require.NoError(t, err)

		// Verify files were created
		cacheDir := filepath.Join(tmpDir, entry.ID())
		assert.DirExists(t, cacheDir)
		assert.FileExists(t, filepath.Join(cacheDir, "profile.yaml"))
		assert.FileExists(t, filepath.Join(cacheDir, "metadata.json"))
		assert.FileExists(t, filepath.Join(cacheDir, "digest.txt"))
	})

	t.Run("find retrieves stored entry", func(t *testing.T) {
		found, err := repo.Find(ctx, ref)

		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, entry.ID(), found.ID())
		assert.Equal(t, content, found.Content())
		assert.Equal(t, contentHash.String(), found.ContentHash().String())
		assert.Equal(t, "\"etag-123\"", found.ETag())
	})

	t.Run("find returns nil for non-existent", func(t *testing.T) {
		nonExistent, _ := values.ParseProfileReference("https://example.com/not-found.yaml")

		found, err := repo.Find(ctx, nonExistent)

		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func Test_FSProfileCacheRepository_List(t *testing.T) {
	tmpDir := t.TempDir()
	repo, err := profiles.NewFSProfileCacheRepository(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("returns empty list initially", func(t *testing.T) {
		entries, err := repo.List(ctx)

		require.NoError(t, err)
		assert.Empty(t, entries)
	})

	t.Run("returns all stored entries", func(t *testing.T) {
		// Store multiple entries
		for i, url := range []string{
			"https://example.com/profile1.yaml",
			"https://example.com/profile2.yaml",
			"https://other.com/profile.yaml",
		} {
			ref, _ := values.ParseProfileReference(url)
			content := []byte("# profile " + string(rune('1'+i)))
			hash, _ := hostValues.ComputeDigestSHA256(strings.NewReader(string(content)))
			entry, err := entities.NewProfileCacheEntry(ref, content, hash, time.Hour)
			require.NoError(t, err)
			require.NoError(t, repo.Store(ctx, entry))
		}

		entries, err := repo.List(ctx)

		require.NoError(t, err)
		assert.Len(t, entries, 3)
	})
}

func Test_FSProfileCacheRepository_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	repo, err := profiles.NewFSProfileCacheRepository(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	// Store an entry
	ref, _ := values.ParseProfileReference("https://example.com/to-delete.yaml")
	content := []byte("# delete me")
	hash, _ := hostValues.ComputeDigestSHA256(strings.NewReader(string(content)))
	entry, err := entities.NewProfileCacheEntry(ref, content, hash, time.Hour)
	require.NoError(t, err)
	require.NoError(t, repo.Store(ctx, entry))

	t.Run("delete removes entry", func(t *testing.T) {
		err := repo.Delete(ctx, ref)
		require.NoError(t, err)

		// Should not be found
		found, err := repo.Find(ctx, ref)
		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("delete non-existent is not an error", func(t *testing.T) {
		nonExistent, _ := values.ParseProfileReference("https://example.com/does-not-exist.yaml")

		err := repo.Delete(ctx, nonExistent)

		require.NoError(t, err)
	})
}

func Test_FSProfileCacheRepository_Prune(t *testing.T) {
	tmpDir := t.TempDir()
	repo, err := profiles.NewFSProfileCacheRepository(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	// Create fresh entry
	freshRef, _ := values.ParseProfileReference("https://example.com/fresh.yaml")
	freshContent := []byte("# fresh")
	freshHash, _ := hostValues.ComputeDigestSHA256(strings.NewReader(string(freshContent)))
	freshEntry, err := entities.NewProfileCacheEntry(freshRef, freshContent, freshHash, time.Hour)
	require.NoError(t, err)
	require.NoError(t, repo.Store(ctx, freshEntry))

	// Create old entry by storing then manipulating the metadata
	oldRef, _ := values.ParseProfileReference("https://example.com/old.yaml")
	oldContent := []byte("# old")
	oldHash, _ := hostValues.ComputeDigestSHA256(strings.NewReader(string(oldContent)))
	oldEntry, err := entities.NewProfileCacheEntry(oldRef, oldContent, oldHash, time.Hour)
	require.NoError(t, err)
	require.NoError(t, repo.Store(ctx, oldEntry))

	// Manually adjust the last accessed time in the metadata file
	oldMetadataPath := filepath.Join(tmpDir, oldEntry.ID(), "metadata.json")
	metadataBytes, err := os.ReadFile(oldMetadataPath)
	require.NoError(t, err)

	// Replace last_accessed_at with an old timestamp (48 hours ago)
	oldTimeStr := time.Now().Add(-48 * time.Hour).Format(time.RFC3339Nano)
	// Simple replacement - find and replace the timestamp
	modifiedMetadata := replaceJSONTimestamp(string(metadataBytes), "last_accessed_at", oldTimeStr)
	require.NoError(t, os.WriteFile(oldMetadataPath, []byte(modifiedMetadata), 0640))

	t.Run("prune removes old entries", func(t *testing.T) {
		pruned, err := repo.Prune(ctx, 24*time.Hour)

		require.NoError(t, err)
		assert.Equal(t, 1, pruned)

		// Fresh should still exist
		found, err := repo.Find(ctx, freshRef)
		require.NoError(t, err)
		assert.NotNil(t, found)

		// Old should be gone
		found, err = repo.Find(ctx, oldRef)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

// replaceJSONTimestamp is a helper to replace a timestamp in JSON metadata.
func replaceJSONTimestamp(json, key, newTime string) string {
	// Find the key and replace its value
	keyPattern := `"` + key + `":`
	idx := strings.Index(json, keyPattern)
	if idx == -1 {
		return json
	}

	// Find the start of the value (after the colon)
	valueStart := idx + len(keyPattern)
	// Skip whitespace
	for valueStart < len(json) && (json[valueStart] == ' ' || json[valueStart] == '\t') {
		valueStart++
	}

	// If it starts with a quote, find the end quote
	if json[valueStart] == '"' {
		valueEnd := strings.Index(json[valueStart+1:], `"`)
		if valueEnd == -1 {
			return json
		}
		valueEnd += valueStart + 2 // Include the closing quote
		return json[:valueStart] + `"` + newTime + `"` + json[valueEnd:]
	}

	return json
}
