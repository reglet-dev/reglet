package profiles

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	hostValues "github.com/reglet-dev/reglet-host-sdk/plugin/values"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/values"
)

// FSProfileCacheRepository implements ProfileCacheRepository using the filesystem.
// Profiles are stored at ~/.reglet/profiles/<cache-key>/
type FSProfileCacheRepository struct {
	// Root is the base directory for the cache.
	// Default: ~/.reglet/profiles
	Root string
}

// NewFSProfileCacheRepository creates a new filesystem-based cache repository.
func NewFSProfileCacheRepository(root string) (*FSProfileCacheRepository, error) {
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		root = filepath.Join(home, ".reglet", "profiles")
	}

	// Ensure root directory exists
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &FSProfileCacheRepository{Root: root}, nil
}

// Find retrieves a cached profile by reference.
func (r *FSProfileCacheRepository) Find(ctx context.Context, ref values.ProfileReference) (*entities.ProfileCacheEntry, error) {
	cacheDir := r.cacheDir(ref)

	// Check if cache directory exists
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		return nil, nil // Not found
	}

	// Read metadata
	metadataPath := filepath.Join(cacheDir, "metadata.json")
	//nolint:gosec // Path is constructed from hash and is within cache root
	metadataBytes, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	var metadata cacheMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	// Read content
	contentPath := filepath.Join(cacheDir, "profile.yaml")
	//nolint:gosec // Path is constructed from hash and is within cache root
	content, err := os.ReadFile(contentPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cached content: %w", err)
	}

	// Parse digest
	contentHash, err := hostValues.ParseDigest(metadata.Digest)
	if err != nil {
		return nil, fmt.Errorf("invalid cached digest: %w", err)
	}

	// Reconstruct cache entry
	entry := entities.LoadProfileCacheEntry(
		ref.CacheKey(),
		ref,
		content,
		contentHash,
		metadata.FetchedAt,
		metadata.LastAccessedAt,
		metadata.TTL,
		metadata.ETag,
	)

	// Update last accessed time
	entry.Touch()
	_ = r.updateLastAccessed(cacheDir, entry.LastAccessedAt())

	return entry, nil
}

// Store persists a profile cache entry.
func (r *FSProfileCacheRepository) Store(ctx context.Context, entry *entities.ProfileCacheEntry) error {
	cacheDir := filepath.Join(r.Root, entry.ID())

	// Validate path to prevent traversal attacks (Constitution II: Path Traversal Protection)
	if err := r.validatePath(cacheDir); err != nil {
		return err
	}

	// Create cache directory
	if err := os.MkdirAll(cacheDir, 0o750); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Write content atomically (temp file + rename)
	contentPath := filepath.Join(cacheDir, "profile.yaml")
	if err := r.writeFileAtomic(contentPath, entry.Content()); err != nil {
		return fmt.Errorf("failed to write content: %w", err)
	}

	// Write metadata
	metadata := cacheMetadata{
		URL:            entry.Reference().String(),
		Digest:         entry.ContentHash().String(),
		FetchedAt:      entry.FetchedAt(),
		LastAccessedAt: entry.LastAccessedAt(),
		TTL:            entry.TTL(),
		ETag:           entry.ETag(),
		Size:           entry.Size(),
	}

	metadataBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize metadata: %w", err)
	}

	metadataPath := filepath.Join(cacheDir, "metadata.json")
	if err := r.writeFileAtomic(metadataPath, metadataBytes); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	// Write digest file for easy inspection
	digestPath := filepath.Join(cacheDir, "digest.txt")
	if err := r.writeFileAtomic(digestPath, []byte(entry.ContentHash().String())); err != nil {
		return fmt.Errorf("failed to write digest: %w", err)
	}

	return nil
}

// List returns all cached profiles.
func (r *FSProfileCacheRepository) List(ctx context.Context) ([]*entities.ProfileCacheEntry, error) {
	entries, err := os.ReadDir(r.Root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read cache directory: %w", err)
	}

	var result []*entities.ProfileCacheEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		cacheDir := filepath.Join(r.Root, entry.Name())
		metadataPath := filepath.Join(cacheDir, "metadata.json")

		//nolint:gosec // Path is from os.ReadDir (safe) and within cache root
		metadataBytes, err := os.ReadFile(metadataPath)
		if err != nil {
			continue // Skip invalid entries
		}

		var metadata cacheMetadata
		if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
			continue // Skip invalid entries
		}

		// Parse the URL to get a reference
		ref, err := values.ParseProfileReference(metadata.URL)
		if err != nil {
			continue // Skip invalid entries
		}

		// Read content
		contentPath := filepath.Join(cacheDir, "profile.yaml")
		//nolint:gosec // Path is from os.ReadDir (safe) and within cache root
		content, err := os.ReadFile(contentPath)
		if err != nil {
			continue // Skip invalid entries
		}

		contentHash, err := hostValues.ParseDigest(metadata.Digest)
		if err != nil {
			continue // Skip invalid entries
		}

		cacheEntry := entities.LoadProfileCacheEntry(
			entry.Name(),
			ref,
			content,
			contentHash,
			metadata.FetchedAt,
			metadata.LastAccessedAt,
			metadata.TTL,
			metadata.ETag,
		)

		result = append(result, cacheEntry)
	}

	return result, nil
}

// Delete removes a specific profile from cache.
func (r *FSProfileCacheRepository) Delete(ctx context.Context, ref values.ProfileReference) error {
	cacheDir := r.cacheDir(ref)

	// Validate path
	if err := r.validatePath(cacheDir); err != nil {
		return err
	}

	if err := os.RemoveAll(cacheDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete cache entry: %w", err)
	}

	return nil
}

// Prune removes profiles older than the specified duration.
func (r *FSProfileCacheRepository) Prune(ctx context.Context, maxAge time.Duration) (int, error) {
	entries, err := r.List(ctx)
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-maxAge)
	pruned := 0

	for _, entry := range entries {
		if entry.LastAccessedAt().Before(cutoff) {
			if err := r.Delete(ctx, entry.Reference()); err == nil {
				pruned++
			}
		}
	}

	return pruned, nil
}

// cacheDir returns the cache directory for a profile reference.
func (r *FSProfileCacheRepository) cacheDir(ref values.ProfileReference) string {
	return filepath.Join(r.Root, ref.CacheKey())
}

// validatePath ensures the path is within the cache root (path traversal protection).
func (r *FSProfileCacheRepository) validatePath(path string) error {
	absRoot, err := filepath.Abs(r.Root)
	if err != nil {
		return fmt.Errorf("failed to resolve root path: %w", err)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Use filepath.Rel to check if path is within root
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return fmt.Errorf("path validation failed: %w", err)
	}

	// Check for path traversal (rel should not start with "..")
	if rel == ".." || len(rel) > 2 && rel[:3] == ".."+string(filepath.Separator) {
		return fmt.Errorf("path traversal attempt blocked: %s", path)
	}

	return nil
}

// writeFileAtomic writes data to a file atomically using temp file + rename.
func (r *FSProfileCacheRepository) writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	// Clean up on failure
	defer func() {
		if tmpPath != "" {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}

	if err := tmp.Chmod(0o640); err != nil {
		_ = tmp.Close()
		return err
	}

	if err := tmp.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}

	tmpPath = "" // Prevent cleanup
	return nil
}

// updateLastAccessed updates the last accessed time in metadata.
func (r *FSProfileCacheRepository) updateLastAccessed(cacheDir string, t time.Time) error {
	metadataPath := filepath.Join(cacheDir, "metadata.json")

	//nolint:gosec // Path is constructed from hash and is within cache root
	metadataBytes, err := os.ReadFile(metadataPath)
	if err != nil {
		return err
	}

	var metadata cacheMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return err
	}

	metadata.LastAccessedAt = t

	updatedBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	return r.writeFileAtomic(metadataPath, updatedBytes)
}

// cacheMetadata is the JSON structure stored in metadata.json.
type cacheMetadata struct {
	FetchedAt      time.Time     `json:"fetched_at"`
	LastAccessedAt time.Time     `json:"last_accessed_at"`
	URL            string        `json:"url"`
	Digest         string        `json:"digest"`
	ETag           string        `json:"etag,omitempty"`
	TTL            time.Duration `json:"ttl"`
	Size           int64         `json:"size"`
}
