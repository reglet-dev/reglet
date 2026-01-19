package entities_test

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/values"
)

func Test_NewProfileCacheEntry_Valid(t *testing.T) {
	ref, err := values.ParseProfileReference("https://example.com/profile.yaml")
	require.NoError(t, err)

	content := []byte("name: test-profile\nversion: 1.0.0")
	hash, err := values.ComputeDigestSHA256(bytesReader(content))
	require.NoError(t, err)

	entry, err := entities.NewProfileCacheEntry(ref, content, hash, time.Hour)
	require.NoError(t, err)

	assert.Equal(t, ref.CacheKey(), entry.ID())
	assert.Equal(t, content, entry.Content())
	assert.Equal(t, hash, entry.ContentHash())
	assert.Equal(t, time.Hour, entry.TTL())
	assert.Equal(t, int64(len(content)), entry.Size())
	assert.True(t, entry.IsFresh())
	assert.False(t, entry.IsExpired())
	assert.False(t, entry.IsStale())
}

func Test_NewProfileCacheEntry_InvalidTTL(t *testing.T) {
	ref, _ := values.ParseProfileReference("https://example.com/profile.yaml")
	content := []byte("test")
	hash, _ := values.ComputeDigestSHA256(bytesReader(content))

	_, err := entities.NewProfileCacheEntry(ref, content, hash, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TTL must be positive")

	_, err = entities.NewProfileCacheEntry(ref, content, hash, -time.Hour)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TTL must be positive")
}

func Test_ProfileCacheEntry_CacheStates(t *testing.T) {
	ref, _ := values.ParseProfileReference("https://example.com/profile.yaml")
	content := []byte("test")
	hash, _ := values.ComputeDigestSHA256(bytesReader(content))

	ttl := time.Hour
	freshTime := time.Now().UTC()
	staleTime := freshTime.Add(-ttl - time.Minute) // Past TTL but within 2x
	expiredTime := freshTime.Add(-ttl * 3)         // Past 2x TTL

	tests := []struct {
		name      string
		fetchedAt time.Time
		wantState entities.CacheState
		wantFresh bool
		wantStale bool
		wantExp   bool
	}{
		{
			name:      "fresh entry",
			fetchedAt: freshTime,
			wantState: entities.CacheStateFresh,
			wantFresh: true,
		},
		{
			name:      "stale entry",
			fetchedAt: staleTime,
			wantState: entities.CacheStateStale,
			wantStale: true,
			wantExp:   true, // IsExpired returns true for any entry past TTL
		},
		{
			name:      "expired entry",
			fetchedAt: expiredTime,
			wantState: entities.CacheStateExpired,
			wantExp:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := entities.LoadProfileCacheEntry(
				ref.CacheKey(),
				ref,
				content,
				hash,
				tt.fetchedAt,
				tt.fetchedAt,
				ttl,
				"",
			)

			assert.Equal(t, tt.wantState, entry.State())
			assert.Equal(t, tt.wantFresh, entry.IsFresh())
			assert.Equal(t, tt.wantStale, entry.IsStale())
			assert.Equal(t, tt.wantExp, entry.IsExpired())
		})
	}
}

func Test_ProfileCacheEntry_Touch(t *testing.T) {
	ref, _ := values.ParseProfileReference("https://example.com/profile.yaml")
	content := []byte("test")
	hash, _ := values.ComputeDigestSHA256(bytesReader(content))

	entry, err := entities.NewProfileCacheEntry(ref, content, hash, time.Hour)
	require.NoError(t, err)

	initialAccess := entry.LastAccessedAt()
	time.Sleep(10 * time.Millisecond)
	entry.Touch()

	assert.True(t, entry.LastAccessedAt().After(initialAccess))
}

func Test_ProfileCacheEntry_ETag(t *testing.T) {
	ref, _ := values.ParseProfileReference("https://example.com/profile.yaml")
	content := []byte("test")
	hash, _ := values.ComputeDigestSHA256(bytesReader(content))

	entry, err := entities.NewProfileCacheEntry(ref, content, hash, time.Hour)
	require.NoError(t, err)

	assert.Empty(t, entry.ETag())
	entry.SetETag("abc123")
	assert.Equal(t, "abc123", entry.ETag())
}

func Test_ProfileCacheEntry_ValidateContent(t *testing.T) {
	ref, _ := values.ParseProfileReference("https://example.com/profile.yaml")
	content := []byte("test content")
	hash, _ := values.ComputeDigestSHA256(bytesReader(content))

	entry, err := entities.NewProfileCacheEntry(ref, content, hash, time.Hour)
	require.NoError(t, err)

	// Valid content should pass
	assert.NoError(t, entry.ValidateContent())
}

func Test_ProfileCacheEntry_StateString(t *testing.T) {
	ref, _ := values.ParseProfileReference("https://example.com/profile.yaml")
	content := []byte("test")
	hash, _ := values.ComputeDigestSHA256(bytesReader(content))

	entry, _ := entities.NewProfileCacheEntry(ref, content, hash, time.Hour)
	assert.Equal(t, "fresh", entry.StateString())
}

func Test_ProfileCacheEntry_Age(t *testing.T) {
	ref, _ := values.ParseProfileReference("https://example.com/profile.yaml")
	content := []byte("test")
	hash, _ := values.ComputeDigestSHA256(bytesReader(content))

	// Create an entry that was fetched 30 minutes ago
	fetchedAt := time.Now().UTC().Add(-30 * time.Minute)
	entry := entities.LoadProfileCacheEntry(
		ref.CacheKey(),
		ref,
		content,
		hash,
		fetchedAt,
		fetchedAt,
		time.Hour,
		"",
	)

	age := entry.Age()
	assert.True(t, age >= 30*time.Minute)
	assert.True(t, age < 31*time.Minute)
}

// bytesReader is a helper to create an io.Reader from bytes.
func bytesReader(data []byte) *bytesReaderImpl {
	return &bytesReaderImpl{data: data, pos: 0}
}

type bytesReaderImpl struct {
	data []byte
	pos  int
}

func (r *bytesReaderImpl) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
