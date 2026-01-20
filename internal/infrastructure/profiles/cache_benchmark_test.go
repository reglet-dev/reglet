package profiles_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/values"
	"github.com/reglet-dev/reglet/internal/infrastructure/profiles"
)

// BenchmarkCacheStore measures the time to store a profile in the cache.
func BenchmarkCacheStore(b *testing.B) {
	// Setup temp directory
	tempDir := b.TempDir()
	repo, err := profiles.NewFSProfileCacheRepository(tempDir)
	if err != nil {
		b.Fatalf("failed to create repo: %v", err)
	}

	// Create a sample entry
	ref, _ := values.ParseProfileReference("https://example.com/benchmark-profile.yaml")
	content := []byte("profile:\n  name: benchmark\n  version: 1.0.0\n")
	hash, _ := values.ComputeDigestSHA256(bytes.NewReader(content))

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entry, _ := entities.NewProfileCacheEntry(ref, content, hash, time.Hour)
		if err := repo.Store(ctx, entry); err != nil {
			b.Fatalf("store failed: %v", err)
		}
	}
}

// BenchmarkCacheFind measures the time to find a profile in the cache.
func BenchmarkCacheFind(b *testing.B) {
	// Setup temp directory with a cached entry
	tempDir := b.TempDir()
	repo, err := profiles.NewFSProfileCacheRepository(tempDir)
	if err != nil {
		b.Fatalf("failed to create repo: %v", err)
	}

	// Create and store a sample entry
	ref, _ := values.ParseProfileReference("https://example.com/benchmark-profile.yaml")
	content := []byte("profile:\n  name: benchmark\n  version: 1.0.0\n")
	hash, _ := values.ComputeDigestSHA256(bytes.NewReader(content))

	ctx := context.Background()
	entry, _ := entities.NewProfileCacheEntry(ref, content, hash, time.Hour)
	if err := repo.Store(ctx, entry); err != nil {
		b.Fatalf("initial store failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := repo.Find(ctx, ref)
		if err != nil {
			b.Fatalf("find failed: %v", err)
		}
	}
}

// BenchmarkCacheList measures the time to list all cached profiles.
func BenchmarkCacheList(b *testing.B) {
	// Setup temp directory with multiple cached entries
	tempDir := b.TempDir()
	repo, err := profiles.NewFSProfileCacheRepository(tempDir)
	if err != nil {
		b.Fatalf("failed to create repo: %v", err)
	}

	ctx := context.Background()
	content := []byte("profile:\n  name: benchmark\n  version: 1.0.0\n")
	hash, _ := values.ComputeDigestSHA256(bytes.NewReader(content))

	// Create 10 cached entries
	for i := 0; i < 10; i++ {
		ref, _ := values.ParseProfileReference("https://example.com/profile-" + string(rune('a'+i)) + ".yaml")
		entry, _ := entities.NewProfileCacheEntry(ref, content, hash, time.Hour)
		if err := repo.Store(ctx, entry); err != nil {
			b.Fatalf("store failed: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := repo.List(ctx)
		if err != nil {
			b.Fatalf("list failed: %v", err)
		}
	}
}

// BenchmarkCacheKeyGeneration measures the time to generate cache keys.
func BenchmarkCacheKeyGeneration(b *testing.B) {
	ref, _ := values.ParseProfileReference("https://example.com/some/path/to/profile.yaml")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ref.CacheKey()
	}
}

// BenchmarkProfileReferenceParsing measures URL parsing performance.
func BenchmarkProfileReferenceParsing(b *testing.B) {
	urls := []string{
		"https://example.com/profile.yaml",
		"https://example.com/profile.yaml#v1.2.0",
		"https://example.com/profile.yaml@sha256:abc123def456",
		"oci://ghcr.io/org/profiles/baseline:v1.0.0",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		url := urls[i%len(urls)]
		_, _ = values.ParseProfileReference(url)
	}
}

// BenchmarkDigestComputation measures hash computation for profile content.
func BenchmarkDigestComputation(b *testing.B) {
	// 10KB profile content
	content := make([]byte, 10*1024)
	for i := range content {
		content[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = values.ComputeDigestSHA256(bytes.NewReader(content))
	}
}

// BenchmarkCacheDirCreation measures directory creation overhead.
func BenchmarkCacheDirCreation(b *testing.B) {
	baseDir := b.TempDir()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dir := filepath.Join(baseDir, "cache-"+string(rune('a'+i%26)))
		_ = os.MkdirAll(dir, 0o750)
	}
}
