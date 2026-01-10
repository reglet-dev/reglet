package filesystem_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/infrastructure/filesystem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileLockfileRepository(t *testing.T) {
	t.Parallel()

	// Temp dir for testing
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "reglet.lock")
	repo := filesystem.NewFileLockfileRepository()
	ctx := context.Background()

	t.Run("Save and Load", func(t *testing.T) {
		lock := entities.NewLockfile()
		lock.Generated = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		err := lock.AddPlugin("test", entities.PluginLock{
			Requested: "1.0",
			Resolved:  "1.0.0",
			Digest:    "sha256:abc",
			Source:    "test-source",
		})
		require.NoError(t, err)

		// Save
		err = repo.Save(ctx, lock, lockPath)
		require.NoError(t, err)

		// Exists
		exists, err := repo.Exists(ctx, lockPath)
		require.NoError(t, err)
		assert.True(t, exists)

		// Load
		loaded, err := repo.Load(ctx, lockPath)
		require.NoError(t, err)
		require.NotNil(t, loaded)

		assert.Equal(t, lock.Version, loaded.Version)
		// Compare timestamps appropriately (serialization might lose sub-second precision or timezone)
		assert.Equal(t, lock.Generated.Unix(), loaded.Generated.Unix())

		plugin := loaded.GetPlugin("test")
		require.NotNil(t, plugin)
		assert.Equal(t, "1.0.0", plugin.Resolved)
		assert.Equal(t, "sha256:abc", plugin.Digest)
	})

	t.Run("Load non-existent", func(t *testing.T) {
		loaded, err := repo.Load(ctx, filepath.Join(tmpDir, "missing.lock"))
		require.NoError(t, err)
		assert.Nil(t, loaded)
	})

	t.Run("Save ensures directory", func(t *testing.T) {
		subdir := filepath.Join(tmpDir, "subdir")
		subLockPath := filepath.Join(subdir, "reglet.lock")

		lock := entities.NewLockfile()
		// Add dummy plugin to satisfy validation (Generated time is set by NewLockfile)
		_ = lock.AddPlugin("dummy", entities.PluginLock{Digest: "d"})

		err := repo.Save(ctx, lock, subLockPath)
		require.NoError(t, err)

		exists, err := repo.Exists(ctx, subLockPath)
		require.NoError(t, err)
		assert.True(t, exists)
	})
}
