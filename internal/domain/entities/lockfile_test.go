package entities_test

import (
	"testing"
	"time"

	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLockfile(t *testing.T) {
	t.Parallel()

	lock := entities.NewLockfile()
	assert.Equal(t, 1, lock.Version)
	assert.False(t, lock.Generated.IsZero())
	assert.Empty(t, lock.Plugins)
}

func TestLockfile_AddPlugin(t *testing.T) {
	t.Parallel()

	t.Run("valid plugin", func(t *testing.T) {
		lock := entities.NewLockfile()
		pluginLock := entities.PluginLock{
			Requested: "@1.0",
			Resolved:  "1.0.2",
			Source:    "https://example.com/plugin.wasm",
			Digest:    "sha256:123456",
			Fetched:   time.Now(),
		}

		err := lock.AddPlugin("reglet/test", pluginLock)
		require.NoError(t, err)
		assert.Equal(t, 1, lock.PluginCount())

		retrieved := lock.GetPlugin("reglet/test")
		require.NotNil(t, retrieved)
		assert.Equal(t, "1.0.2", retrieved.Resolved)
	})

	t.Run("missing digest", func(t *testing.T) {
		lock := entities.NewLockfile()
		pluginLock := entities.PluginLock{
			Requested: "@1.0",
			Resolved:  "1.0.2",
			// Digest missing
		}

		err := lock.AddPlugin("reglet/test", pluginLock)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "digest is required")
		assert.Equal(t, 0, lock.PluginCount())
	})
}

func TestLockfile_Validate(t *testing.T) {
	t.Parallel()

	t.Run("valid, empty", func(t *testing.T) {
		lock := entities.NewLockfile()
		assert.NoError(t, lock.Validate())
	})

	t.Run("valid, populated", func(t *testing.T) {
		lock := entities.NewLockfile()
		_ = lock.AddPlugin("p1", entities.PluginLock{Digest: "hash"})
		assert.NoError(t, lock.Validate())
	})

	t.Run("missing timestamp with plugins", func(t *testing.T) {
		lock := entities.NewLockfile()
		_ = lock.AddPlugin("p1", entities.PluginLock{Digest: "hash"})
		lock.Generated = time.Time{} // Clear timestamp
		assert.ErrorContains(t, lock.Validate(), "generated timestamp is required")
	})

	t.Run("missing timestamp with profiles", func(t *testing.T) {
		lock := entities.NewLockfile()
		_ = lock.AddProfile("https://example.com/profile.yaml", entities.ProfileLock{Digest: "hash"})
		lock.Generated = time.Time{} // Clear timestamp
		assert.ErrorContains(t, lock.Validate(), "generated timestamp is required")
	})
}

func TestLockfile_AddProfile(t *testing.T) {
	t.Parallel()

	t.Run("valid profile", func(t *testing.T) {
		lock := entities.NewLockfile()
		profileLock := entities.ProfileLock{
			Requested: "https://example.com/profile.yaml#v1.2.0",
			Resolved:  "v1.2.0",
			Source:    "https://example.com/profile.yaml",
			Digest:    "sha256:abc123",
			Fetched:   time.Now(),
		}

		err := lock.AddProfile("https://example.com/profile.yaml", profileLock)
		require.NoError(t, err)
		assert.Equal(t, 1, lock.ProfileCount())

		retrieved := lock.GetProfile("https://example.com/profile.yaml")
		require.NotNil(t, retrieved)
		assert.Equal(t, "v1.2.0", retrieved.Resolved)
	})

	t.Run("missing digest", func(t *testing.T) {
		lock := entities.NewLockfile()
		profileLock := entities.ProfileLock{
			Requested: "https://example.com/profile.yaml",
			// Digest missing
		}

		err := lock.AddProfile("https://example.com/profile.yaml", profileLock)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "digest is required")
		assert.Equal(t, 0, lock.ProfileCount())
	})

	t.Run("get nonexistent profile", func(t *testing.T) {
		lock := entities.NewLockfile()
		retrieved := lock.GetProfile("https://nonexistent.com/profile.yaml")
		assert.Nil(t, retrieved)
	})
}
