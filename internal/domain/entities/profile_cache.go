package entities

import (
	"fmt"
	"time"

	"github.com/reglet-dev/reglet/internal/domain/values"
)

// ProfileCacheEntry is an aggregate root representing a cached remote profile.
// It contains the profile content along with metadata for cache management.
//
// Invariants:
//   - contentHash must match SHA256(content)
//   - fetchedAt must not be zero
//   - ttl must be positive
//   - size must equal len(content)
type ProfileCacheEntry struct {
	id             string                  // Cache key (hash of URL)
	reference      values.ProfileReference // Original reference
	content        []byte                  // Profile YAML content
	contentHash    values.Digest           // SHA256 of content
	fetchedAt      time.Time               // When first fetched
	lastAccessedAt time.Time               // Last time this entry was accessed
	ttl            time.Duration           // Cache validity period
	etag           string                  // HTTP ETag for update checks (optional)
	size           int64                   // Content size in bytes
}

// NewProfileCacheEntry creates a new cache entry with validated invariants.
func NewProfileCacheEntry(
	ref values.ProfileReference,
	content []byte,
	contentHash values.Digest,
	ttl time.Duration,
) (*ProfileCacheEntry, error) {
	now := time.Now().UTC()

	entry := &ProfileCacheEntry{
		id:             ref.CacheKey(),
		reference:      ref,
		content:        content,
		contentHash:    contentHash,
		fetchedAt:      now,
		lastAccessedAt: now,
		ttl:            ttl,
		size:           int64(len(content)),
	}

	if err := entry.Validate(); err != nil {
		return nil, err
	}

	return entry, nil
}

// LoadProfileCacheEntry reconstructs an entry from stored data (e.g., from disk).
// Does not validate content hash (assumes already validated).
func LoadProfileCacheEntry(
	id string,
	ref values.ProfileReference,
	content []byte,
	contentHash values.Digest,
	fetchedAt time.Time,
	lastAccessedAt time.Time,
	ttl time.Duration,
	etag string,
) *ProfileCacheEntry {
	return &ProfileCacheEntry{
		id:             id,
		reference:      ref,
		content:        content,
		contentHash:    contentHash,
		fetchedAt:      fetchedAt,
		lastAccessedAt: lastAccessedAt,
		ttl:            ttl,
		etag:           etag,
		size:           int64(len(content)),
	}
}

// ID returns the cache key.
func (e *ProfileCacheEntry) ID() string {
	return e.id
}

// Reference returns the original profile reference.
func (e *ProfileCacheEntry) Reference() values.ProfileReference {
	return e.reference
}

// Content returns the cached profile content.
func (e *ProfileCacheEntry) Content() []byte {
	return e.content
}

// ContentHash returns the content digest.
func (e *ProfileCacheEntry) ContentHash() values.Digest {
	return e.contentHash
}

// FetchedAt returns when the profile was originally fetched.
func (e *ProfileCacheEntry) FetchedAt() time.Time {
	return e.fetchedAt
}

// LastAccessedAt returns when the entry was last accessed.
func (e *ProfileCacheEntry) LastAccessedAt() time.Time {
	return e.lastAccessedAt
}

// TTL returns the cache validity period.
func (e *ProfileCacheEntry) TTL() time.Duration {
	return e.ttl
}

// ETag returns the HTTP ETag if available.
func (e *ProfileCacheEntry) ETag() string {
	return e.etag
}

// Size returns the content size in bytes.
func (e *ProfileCacheEntry) Size() int64 {
	return e.size
}

// SetETag sets the HTTP ETag for update checking.
func (e *ProfileCacheEntry) SetETag(etag string) {
	e.etag = etag
}

// IsExpired returns true if the cache entry has exceeded its TTL.
// Expired entries should be re-fetched before use.
func (e *ProfileCacheEntry) IsExpired() bool {
	return time.Now().UTC().After(e.fetchedAt.Add(e.ttl))
}

// IsStale returns true if the entry is past its TTL but within the stale period.
// Stale entries can be used but should trigger an async update check.
// The stale period is 2x the TTL.
func (e *ProfileCacheEntry) IsStale() bool {
	now := time.Now().UTC()
	freshUntil := e.fetchedAt.Add(e.ttl)
	staleUntil := e.fetchedAt.Add(e.ttl * 2)
	return now.After(freshUntil) && now.Before(staleUntil)
}

// IsFresh returns true if the entry is within its TTL and valid for use.
func (e *ProfileCacheEntry) IsFresh() bool {
	return time.Now().UTC().Before(e.fetchedAt.Add(e.ttl))
}

// Touch updates the last accessed timestamp.
func (e *ProfileCacheEntry) Touch() {
	e.lastAccessedAt = time.Now().UTC()
}

// Age returns how long since the entry was fetched.
func (e *ProfileCacheEntry) Age() time.Duration {
	return time.Since(e.fetchedAt)
}

// Validate checks all invariants.
func (e *ProfileCacheEntry) Validate() error {
	if e.id == "" {
		return fmt.Errorf("cache entry id is required")
	}
	if e.fetchedAt.IsZero() {
		return fmt.Errorf("fetchedAt timestamp is required")
	}
	if e.ttl <= 0 {
		return fmt.Errorf("TTL must be positive, got %v", e.ttl)
	}
	if e.size != int64(len(e.content)) {
		return fmt.Errorf("size mismatch: stored %d, actual %d", e.size, len(e.content))
	}
	return nil
}

// ValidateContent verifies that the content matches the stored hash.
func (e *ProfileCacheEntry) ValidateContent() error {
	return e.contentHash.Verify(e.content)
}

// ExpiresAt returns when this entry will expire.
func (e *ProfileCacheEntry) ExpiresAt() time.Time {
	return e.fetchedAt.Add(e.ttl)
}

// TimeUntilExpiry returns the duration until expiry (negative if expired).
func (e *ProfileCacheEntry) TimeUntilExpiry() time.Duration {
	return time.Until(e.fetchedAt.Add(e.ttl))
}

// CacheState represents the current state of a cache entry.
type CacheState int

const (
	CacheStateFresh   CacheState = iota // Entry is valid and fresh
	CacheStateStale                     // Entry is past TTL but usable
	CacheStateExpired                   // Entry should be re-fetched
)

// State returns the current cache state.
func (e *ProfileCacheEntry) State() CacheState {
	if e.IsFresh() {
		return CacheStateFresh
	}
	if e.IsStale() {
		return CacheStateStale
	}
	return CacheStateExpired
}

// StateString returns a human-readable cache state.
func (e *ProfileCacheEntry) StateString() string {
	switch e.State() {
	case CacheStateFresh:
		return "fresh"
	case CacheStateStale:
		return "stale"
	case CacheStateExpired:
		return "expired"
	default:
		return "unknown"
	}
}
