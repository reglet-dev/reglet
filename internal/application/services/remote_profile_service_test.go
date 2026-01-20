package services_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/application/services"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/values"
)

// mockProfileFetcher implements ports.ProfileFetcher for testing.
type mockProfileFetcher struct {
	content     []byte
	contentHash values.Digest
	etag        string
	fetchCount  int
	fetchErr    error
}

func (m *mockProfileFetcher) Fetch(ctx context.Context, ref values.ProfileReference, opts ports.FetchOptions) (*ports.FetchResult, error) {
	m.fetchCount++
	if m.fetchErr != nil {
		return nil, m.fetchErr
	}
	return &ports.FetchResult{
		Content:     m.content,
		ContentHash: m.contentHash,
		ETag:        m.etag,
	}, nil
}

// mockProfileCacheRepository implements ports.ProfileCacheRepository for testing.
type mockProfileCacheRepository struct {
	entries    map[string]*entities.ProfileCacheEntry
	storeCount int
	storeErr   error
	findErr    error
}

func newMockCache() *mockProfileCacheRepository {
	return &mockProfileCacheRepository{
		entries: make(map[string]*entities.ProfileCacheEntry),
	}
}

func (m *mockProfileCacheRepository) Find(ctx context.Context, ref values.ProfileReference) (*entities.ProfileCacheEntry, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.entries[ref.CacheKey()], nil
}

func (m *mockProfileCacheRepository) Store(ctx context.Context, entry *entities.ProfileCacheEntry) error {
	m.storeCount++
	if m.storeErr != nil {
		return m.storeErr
	}
	m.entries[entry.ID()] = entry
	return nil
}

func (m *mockProfileCacheRepository) List(ctx context.Context) ([]*entities.ProfileCacheEntry, error) {
	var result []*entities.ProfileCacheEntry
	for _, e := range m.entries {
		result = append(result, e)
	}
	return result, nil
}

func (m *mockProfileCacheRepository) Delete(ctx context.Context, ref values.ProfileReference) error {
	delete(m.entries, ref.CacheKey())
	return nil
}

func (m *mockProfileCacheRepository) Prune(ctx context.Context, maxAge time.Duration) (int, error) {
	return 0, nil
}

func Test_IsRemoteProfile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"https://example.com/profile.yaml", true},
		{"HTTPS://EXAMPLE.COM/profile.yaml", true},
		{"oci://ghcr.io/org/profile:v1", true},
		{"OCI://ghcr.io/org/profile", true},
		{"http://example.com/profile.yaml", false}, // HTTP not allowed
		{"./profile.yaml", false},
		{"/path/to/profile.yaml", false},
		{"profile.yaml", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.expected, services.IsRemoteProfile(tt.path))
		})
	}
}

func Test_RemoteProfileService_Fetch(t *testing.T) {
	ctx := context.Background()

	profileContent := []byte("profile:\n  name: test\n  version: 1.0.0\n")
	contentHash, _ := values.ComputeDigestSHA256(strings.NewReader(string(profileContent)))

	t.Run("fetches from network when no cache", func(t *testing.T) {
		fetcher := &mockProfileFetcher{
			content:     profileContent,
			contentHash: contentHash,
		}

		svc := services.NewRemoteProfileService(fetcher)

		result, err := svc.Fetch(ctx, "https://example.com/profile.yaml", services.RemoteFetchOptions{})

		require.NoError(t, err)
		assert.Equal(t, profileContent, result.Content)
		assert.Equal(t, contentHash.String(), result.ContentHash.String())
		assert.False(t, result.FromCache)
		assert.Equal(t, 1, fetcher.fetchCount)
	})

	t.Run("uses cache when available", func(t *testing.T) {
		fetcher := &mockProfileFetcher{
			content:     profileContent,
			contentHash: contentHash,
		}
		cache := newMockCache()

		// Pre-populate cache
		ref, _ := values.ParseProfileReference("https://example.com/cached.yaml")
		entry, _ := entities.NewProfileCacheEntry(ref, profileContent, contentHash, time.Hour)
		cache.entries[ref.CacheKey()] = entry

		svc := services.NewRemoteProfileService(fetcher, services.WithCache(cache))

		result, err := svc.Fetch(ctx, "https://example.com/cached.yaml", services.RemoteFetchOptions{})

		require.NoError(t, err)
		assert.Equal(t, profileContent, result.Content)
		assert.True(t, result.FromCache)
		assert.Equal(t, 0, fetcher.fetchCount) // Should not have called fetcher
	})

	t.Run("bypasses cache when refresh requested", func(t *testing.T) {
		fetcher := &mockProfileFetcher{
			content:     profileContent,
			contentHash: contentHash,
		}
		cache := newMockCache()

		// Pre-populate cache
		ref, _ := values.ParseProfileReference("https://example.com/cached.yaml")
		entry, _ := entities.NewProfileCacheEntry(ref, profileContent, contentHash, time.Hour)
		cache.entries[ref.CacheKey()] = entry

		svc := services.NewRemoteProfileService(fetcher, services.WithCache(cache))

		result, err := svc.Fetch(ctx, "https://example.com/cached.yaml", services.RemoteFetchOptions{
			Refresh: true,
		})

		require.NoError(t, err)
		assert.False(t, result.FromCache)
		assert.Equal(t, 1, fetcher.fetchCount) // Should have called fetcher
		assert.Equal(t, 1, cache.storeCount)   // Should have stored new entry
	})

	t.Run("stores in cache after fetch", func(t *testing.T) {
		fetcher := &mockProfileFetcher{
			content:     profileContent,
			contentHash: contentHash,
			etag:        "\"abc123\"",
		}
		cache := newMockCache()

		svc := services.NewRemoteProfileService(fetcher, services.WithCache(cache))

		_, err := svc.Fetch(ctx, "https://example.com/new.yaml", services.RemoteFetchOptions{})

		require.NoError(t, err)
		assert.Equal(t, 1, cache.storeCount)
		assert.Len(t, cache.entries, 1)
	})

	t.Run("returns error for invalid URL", func(t *testing.T) {
		fetcher := &mockProfileFetcher{}
		svc := services.NewRemoteProfileService(fetcher)

		_, err := svc.Fetch(ctx, "not-a-url", services.RemoteFetchOptions{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid profile URL")
	})

	t.Run("uses stale cache when network fails (offline fallback)", func(t *testing.T) {
		fetcher := &mockProfileFetcher{
			fetchErr: assert.AnError, // Network failure
		}
		cache := newMockCache()

		// Pre-populate cache with a stale entry (TTL expired)
		ref, _ := values.ParseProfileReference("https://example.com/stale.yaml")
		// Create entry with very short TTL that's already expired
		entry, _ := entities.NewProfileCacheEntry(ref, profileContent, contentHash, time.Nanosecond)
		// Sleep briefly to ensure entry is stale
		time.Sleep(time.Millisecond)
		cache.entries[ref.CacheKey()] = entry

		svc := services.NewRemoteProfileService(fetcher, services.WithCache(cache))

		result, err := svc.Fetch(ctx, "https://example.com/stale.yaml", services.RemoteFetchOptions{})

		require.NoError(t, err) // Should NOT error due to offline fallback
		assert.Equal(t, profileContent, result.Content)
		assert.True(t, result.FromCache)
		assert.Equal(t, 1, fetcher.fetchCount) // Should have tried network
	})

	t.Run("returns error when network fails and no cache available", func(t *testing.T) {
		fetcher := &mockProfileFetcher{
			fetchErr: assert.AnError, // Network failure
		}
		cache := newMockCache() // Empty cache

		svc := services.NewRemoteProfileService(fetcher, services.WithCache(cache))

		_, err := svc.Fetch(ctx, "https://example.com/nocache.yaml", services.RemoteFetchOptions{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to fetch profile")
	})
}

func Test_RemoteProfileService_FetchAsReader(t *testing.T) {
	ctx := context.Background()

	profileContent := []byte("profile:\n  name: test\n  version: 1.0.0\n")
	contentHash, _ := values.ComputeDigestSHA256(strings.NewReader(string(profileContent)))

	fetcher := &mockProfileFetcher{
		content:     profileContent,
		contentHash: contentHash,
	}

	svc := services.NewRemoteProfileService(fetcher)

	reader, err := svc.FetchAsReader(ctx, "https://example.com/profile.yaml", services.RemoteFetchOptions{})

	require.NoError(t, err)

	// Read all content
	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, profileContent, content)
}
