// Package profiles provides infrastructure for remote profile fetching and caching.
package profiles

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/domain/values"
	"github.com/reglet-dev/reglet/internal/pkg/netutil"
)

// HTTPProfileFetcher fetches profiles over HTTPS using secure defaults.
// It implements the ports.ProfileFetcher interface.
type HTTPProfileFetcher struct {
	// OnRedirect is called when a redirect is followed.
	// Returns an error to abort the redirect.
	OnRedirect func(req *http.Request, via []*http.Request) error

	// OnPrivateIPWarning is called when SSRF protection blocks a private IP.
	OnPrivateIPWarning func(ip string)

	// OnDNSPinning is called when DNS is resolved and pinned.
	OnDNSPinning func(host string, ip string)

	// OnContentTypeWarning is called when Content-Type is unexpected.
	OnContentTypeWarning func(contentType string)

	// OnRetry is called before each retry attempt.
	OnRetry func(attempt int, statusCode int)

	// UserAgent is the User-Agent header sent with requests.
	UserAgent string
}

// NewHTTPProfileFetcher creates a new HTTP profile fetcher with default settings.
func NewHTTPProfileFetcher() *HTTPProfileFetcher {
	return &HTTPProfileFetcher{
		UserAgent: "reglet-profile-fetcher/1.0",
	}
}

// Fetch retrieves profile content from the given HTTPS URL.
func (f *HTTPProfileFetcher) Fetch(ctx context.Context, ref values.ProfileReference, opts ports.FetchOptions) (*ports.FetchResult, error) {
	if !ref.IsHTTPS() {
		return nil, fmt.Errorf("HTTPProfileFetcher only supports HTTPS URLs, got: %s", ref.Scheme())
	}

	opts = f.applyDefaults(opts)
	client := f.createClient(opts)

	req, reqURL, err := f.createRequest(ctx, ref, opts)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		if netutil.IsPrivateIPError(err) {
			return nil, err
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return f.handleResponse(resp, reqURL, ref, opts)
}

func (f *HTTPProfileFetcher) applyDefaults(opts ports.FetchOptions) ports.FetchOptions {
	if opts.MaxSize == 0 {
		opts.MaxSize = 10 * 1024 * 1024 // 10MB
	}
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.MaxRetries == 0 {
		opts.MaxRetries = 3
	}
	return opts
}

func (f *HTTPProfileFetcher) createClient(opts ports.FetchOptions) *http.Client {
	dialer := &netutil.SecureDialer{
		AllowPrivateNetwork: opts.AllowPrivateNetwork,
		Timeout:             opts.Timeout,
		OnPrivateIPBlocked: func(ip net.IP) {
			if f.OnPrivateIPWarning != nil {
				f.OnPrivateIPWarning(ip.String())
			}
		},
		OnDNSPinning: func(host string, ip net.IP) {
			if f.OnDNSPinning != nil {
				f.OnDNSPinning(host, ip.String())
			}
		},
	}

	tlsConfig := netutil.TLSConfig()
	if opts.Insecure {
		tlsConfig = netutil.InsecureTLSConfig()
	}

	baseTransport := &http.Transport{
		DialContext:     dialer.DialContext,
		TLSClientConfig: tlsConfig,
	}

	retryTransport := &netutil.RetryTransport{
		Base:           baseTransport,
		MaxRetries:     opts.MaxRetries,
		InitialBackoff: time.Second,
		MaxBackoff:     30 * time.Second,
		OnRetry: func(attempt int, _ time.Duration, statusCode int) {
			if f.OnRetry != nil {
				f.OnRetry(attempt, statusCode)
			}
		},
	}

	return &http.Client{
		Transport:     retryTransport,
		Timeout:       opts.Timeout,
		CheckRedirect: f.checkRedirect,
	}
}

func (f *HTTPProfileFetcher) checkRedirect(req *http.Request, via []*http.Request) error {
	// Limit redirects to 5 hops (as per spec)
	if len(via) >= 5 {
		return fmt.Errorf("too many redirects (max 5)")
	}

	// Warn on cross-domain redirects
	if len(via) > 0 && req.URL.Host != via[len(via)-1].URL.Host {
		// Different domain
		if f.OnRedirect != nil {
			if err := f.OnRedirect(req, via); err != nil {
				return err
			}
		}
	}

	return nil
}

func (f *HTTPProfileFetcher) createRequest(ctx context.Context, ref values.ProfileReference, opts ports.FetchOptions) (*http.Request, string, error) {
	reqURL := ref.String()
	// Remove fragment for HTTP request (fragments are client-side only)
	if idx := strings.Index(reqURL, "#"); idx != -1 {
		reqURL = reqURL[:idx]
	}
	// Remove digest suffix (handled separately)
	if idx := strings.Index(reqURL, "@sha256:"); idx != -1 {
		reqURL = reqURL[:idx]
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("User-Agent", f.UserAgent)
	req.Header.Set("Accept", "text/yaml, application/yaml, application/x-yaml, */*")

	// Add custom headers from options
	for k, v := range opts.Headers {
		req.Header.Set(k, v)
	}

	return req, reqURL, nil
}

func (f *HTTPProfileFetcher) handleResponse(resp *http.Response, reqURL string, ref values.ProfileReference, opts ports.FetchOptions) (*ports.FetchResult, error) {
	// Check status
	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			URL:        reqURL,
		}
	}

	// Check Content-Type (warn but don't fail)
	contentType := resp.Header.Get("Content-Type")
	if !isYAMLContentType(contentType) && f.OnContentTypeWarning != nil {
		f.OnContentTypeWarning(contentType)
	}

	// Read body with size limit
	limitedReader := netutil.NewLimitedReader(resp.Body, opts.MaxSize)
	content, err := io.ReadAll(limitedReader)
	if err != nil {
		if netutil.IsSizeLimitExceededError(err) {
			return nil, fmt.Errorf("profile too large: %w (max %s)", err, netutil.FormatSize(opts.MaxSize))
		}
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Compute content hash
	contentHash, err := values.ComputeDigestSHA256(strings.NewReader(string(content)))
	if err != nil {
		return nil, fmt.Errorf("failed to compute content hash: %w", err)
	}

	// Validate against specified digest if present
	if ref.HasDigest() {
		if !ref.Digest().Equals(contentHash) {
			return nil, fmt.Errorf("content hash mismatch: expected %s, got %s",
				ref.Digest().String(), contentHash.String())
		}
	}

	// Count redirects
	redirectCount := 0
	if resp.Request != nil && resp.Request.URL.String() != reqURL {
		// We were redirected
		redirectCount = 1 // Simplified - actual count would require tracking
	}

	return &ports.FetchResult{
		Content:       content,
		ContentHash:   contentHash,
		ETag:          resp.Header.Get("ETag"),
		ContentType:   contentType,
		FinalURL:      resp.Request.URL.String(),
		RedirectCount: redirectCount,
	}, nil
}

// isYAMLContentType checks if the Content-Type indicates YAML content.
func isYAMLContentType(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "yaml") ||
		strings.Contains(ct, "text/plain") ||
		ct == "" // Empty is acceptable
}

// HTTPError represents an HTTP error response.
type HTTPError struct {
	Status     string
	URL        string
	StatusCode int
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s (URL: %s)", e.StatusCode, e.Status, netutil.StripCredentials(e.URL))
}

// IsHTTPError returns true if the error is an HTTPError.
func IsHTTPError(err error) bool {
	hTTPError := &HTTPError{}
	ok := errors.As(err, &hTTPError)
	return ok
}

// GetHTTPStatusCode returns the status code if the error is an HTTPError, or 0.
func GetHTTPStatusCode(err error) int {
	httpErr := &HTTPError{}
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode
	}
	return 0
}
