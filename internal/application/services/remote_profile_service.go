package services

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/values"
)

// RemoteProfileService handles fetching and loading profiles from remote URLs.
// It uses the ProfileFetcher port for HTTP/OCI fetching and integrates with
// the profile cache for performance and offline support.
type RemoteProfileService struct {
	fetcher ports.ProfileFetcher
	cache   ports.ProfileCacheRepository
	logger  *slog.Logger

	// DefaultTTL is the cache TTL for fetched profiles.
	DefaultTTL time.Duration

	// OnFetchStart is called when a fetch operation begins.
	OnFetchStart func(url string)

	// OnFetchComplete is called when a fetch operation completes.
	OnFetchComplete func(url string, cached bool)
}

// RemoteProfileServiceOption configures a RemoteProfileService.
type RemoteProfileServiceOption func(*RemoteProfileService)

// NewRemoteProfileService creates a new remote profile service.
func NewRemoteProfileService(
	fetcher ports.ProfileFetcher,
	opts ...RemoteProfileServiceOption,
) *RemoteProfileService {
	s := &RemoteProfileService{
		fetcher:    fetcher,
		DefaultTTL: time.Hour,
		logger:     slog.Default(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// WithCache sets the profile cache repository.
func WithCache(cache ports.ProfileCacheRepository) RemoteProfileServiceOption {
	return func(s *RemoteProfileService) { s.cache = cache }
}

// WithRemoteLogger sets the logger.
func WithRemoteLogger(logger *slog.Logger) RemoteProfileServiceOption {
	return func(s *RemoteProfileService) { s.logger = logger }
}

// WithDefaultTTL sets the default cache TTL.
func WithDefaultTTL(ttl time.Duration) RemoteProfileServiceOption {
	return func(s *RemoteProfileService) { s.DefaultTTL = ttl }
}

// FetchOptions configures a fetch operation.
type RemoteFetchOptions struct {
	// Refresh forces a cache bypass and re-fetch.
	Refresh bool

	// AllowPrivateNetwork permits fetching from private IP addresses.
	AllowPrivateNetwork bool

	// Timeout overrides the default fetch timeout.
	Timeout time.Duration

	// Insecure allows TLS certificate validation to be skipped.
	Insecure bool

	// Headers are custom HTTP headers to send with the request.
	Headers map[string]string
}

// FetchResult contains the result of fetching a remote profile.
type RemoteFetchResult struct {
	// Content is the raw profile YAML content.
	Content []byte

	// ContentHash is the SHA256 digest of the content.
	ContentHash values.Digest

	// Reference is the parsed profile reference.
	Reference values.ProfileReference

	// FromCache indicates if the content came from cache.
	FromCache bool

	// FetchedAt is when the content was fetched (or cache entry created).
	FetchedAt time.Time
}

// IsRemoteProfile returns true if the path looks like a remote URL.
func IsRemoteProfile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "oci://")
}

// Fetch retrieves a profile from a remote URL.
// It checks the cache first (unless Refresh is true), then fetches from the network.
func (s *RemoteProfileService) Fetch(
	ctx context.Context,
	urlString string,
	opts RemoteFetchOptions,
) (*RemoteFetchResult, error) {
	// Parse the URL into a ProfileReference
	ref, err := values.ParseProfileReference(urlString)
	if err != nil {
		return nil, fmt.Errorf("invalid profile URL: %w", err)
	}

	// Notify fetch start
	if s.OnFetchStart != nil {
		s.OnFetchStart(urlString)
	}

	// Check cache (unless refresh requested)
	if !opts.Refresh && s.cache != nil {
		entry, err := s.cache.Find(ctx, ref)
		if err != nil {
			s.logger.Warn("cache lookup failed", "error", err)
			// Continue to fetch
		} else if entry != nil && entry.IsFresh() {
			s.logger.Debug("using cached profile",
				"url", urlString,
				"age", entry.Age(),
				"expires_in", entry.TimeUntilExpiry())

			if s.OnFetchComplete != nil {
				s.OnFetchComplete(urlString, true)
			}

			return &RemoteFetchResult{
				Content:     entry.Content(),
				ContentHash: entry.ContentHash(),
				Reference:   ref,
				FromCache:   true,
				FetchedAt:   entry.FetchedAt(),
			}, nil
		} else if entry != nil {
			s.logger.Debug("cache entry stale/expired, re-fetching",
				"url", urlString,
				"state", entry.StateString())
		}
	}

	// Fetch from network
	s.logger.Info("fetching remote profile", "url", urlString)

	fetchOpts := ports.FetchOptions{
		AllowPrivateNetwork: opts.AllowPrivateNetwork,
		Timeout:             opts.Timeout,
		Insecure:            opts.Insecure,
		Headers:             opts.Headers,
	}

	result, err := s.fetcher.Fetch(ctx, ref, fetchOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch profile: %w", err)
	}

	// Store in cache
	if s.cache != nil {
		if err := s.storeInCache(ctx, ref, result); err != nil {
			s.logger.Warn("failed to cache profile", "error", err)
			// Non-fatal, continue
		}
	}

	if s.OnFetchComplete != nil {
		s.OnFetchComplete(urlString, false)
	}

	return &RemoteFetchResult{
		Content:     result.Content,
		ContentHash: result.ContentHash,
		Reference:   ref,
		FromCache:   false,
		FetchedAt:   time.Now(),
	}, nil
}

// storeInCache stores a fetch result in the cache.
func (s *RemoteProfileService) storeInCache(
	ctx context.Context,
	ref values.ProfileReference,
	result *ports.FetchResult,
) error {
	entry, err := entities.NewProfileCacheEntry(
		ref,
		result.Content,
		result.ContentHash,
		s.DefaultTTL,
	)
	if err != nil {
		return err
	}

	if result.ETag != "" {
		entry.SetETag(result.ETag)
	}

	return s.cache.Store(ctx, entry)
}

// FetchAsReader fetches a profile and returns it as an io.Reader.
// This is useful for integration with ProfileLoader.LoadProfileFromReader.
func (s *RemoteProfileService) FetchAsReader(
	ctx context.Context,
	urlString string,
	opts RemoteFetchOptions,
) (io.Reader, error) {
	result, err := s.Fetch(ctx, urlString, opts)
	if err != nil {
		return nil, err
	}
	return strings.NewReader(string(result.Content)), nil
}
