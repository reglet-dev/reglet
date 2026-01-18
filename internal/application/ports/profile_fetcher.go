package ports

import (
	"context"
	"time"

	"github.com/reglet-dev/reglet/internal/domain/values"
)

// ProfileFetcher fetches remote profile content from HTTPS URLs or OCI registries.
type ProfileFetcher interface {
	// Fetch retrieves profile content from the given reference.
	// Returns the fetch result containing content, hash, and metadata.
	Fetch(ctx context.Context, ref values.ProfileReference, opts FetchOptions) (*FetchResult, error)
}

// FetchOptions configures profile fetching behavior.
type FetchOptions struct {
	// Headers contains custom HTTP headers (e.g., Authorization).
	Headers map[string]string

	// Insecure allows invalid TLS certificates when true.
	Insecure bool

	// MaxSize limits the response body size in bytes.
	// Default: 10MB if zero.
	MaxSize int64

	// Timeout for the fetch operation.
	// Default: 30s if zero.
	Timeout time.Duration

	// MaxRetries for transient failures (429, 5xx).
	// Default: 3 if zero.
	MaxRetries int

	// AllowPrivateNetwork permits fetching from private IP ranges.
	// Default: false (blocked with warning).
	AllowPrivateNetwork bool
}

// FetchResult contains the result of a profile fetch operation.
type FetchResult struct {
	// Content is the raw profile YAML content.
	Content []byte

	// ContentHash is the SHA256 digest of the content.
	ContentHash values.Digest

	// ETag is the HTTP ETag header value for cache validation.
	ETag string

	// ContentType is the response Content-Type header.
	ContentType string

	// FinalURL is the URL after any redirects.
	FinalURL string

	// RedirectCount is the number of redirects followed.
	RedirectCount int
}

// DefaultFetchOptions returns sensible defaults for fetch operations.
func DefaultFetchOptions() FetchOptions {
	return FetchOptions{
		MaxSize:             10 * 1024 * 1024, // 10MB
		Timeout:             30 * time.Second,
		MaxRetries:          3,
		AllowPrivateNetwork: false,
	}
}
