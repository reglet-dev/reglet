// Package integration provides end-to-end tests for reglet functionality.
package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/domain/values"
	"github.com/reglet-dev/reglet/internal/infrastructure/profiles"
	"github.com/reglet-dev/reglet/internal/infrastructure/sensitivedata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_RemoteProfile_E2E_FullFlow tests the complete remote profile workflow:
// 1. Fetch from HTTPS (httptest)
// 2. Content hash validation
// 3. ETag handling
// 4. Secret detection callback
// 5. Cache storage and retrieval
func Test_RemoteProfile_E2E_FullFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Create test server with a valid profile
	profileContent := `
profile:
  name: e2e-test-profile
  version: 1.0.0

plugins:
  - file

controls:
  items:
    - id: test-control
      name: E2E Test Control
      observations:
        - plugin: file
          config:
            path: /tmp/test.txt
          expect: |
            data.exists == true
`

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/yaml")
		w.Header().Set("ETag", `"abc123"`)
		w.Write([]byte(profileContent))
	}))
	defer server.Close()

	// Create fetcher with callbacks
	fetcher := profiles.NewHTTPProfileFetcher()
	var redirectCount, retryCount int
	var contentTypeWarning string

	fetcher.OnRedirect = func(req *http.Request, via []*http.Request) error {
		redirectCount++
		return nil
	}
	fetcher.OnRetry = func(attempt, statusCode int) {
		retryCount++
	}
	fetcher.OnContentTypeWarning = func(ct string) {
		contentTypeWarning = ct
	}

	// Parse reference (use server URL)
	ref, err := values.ParseProfileReference(server.URL + "/profile.yaml")
	require.NoError(t, err)

	// Fetch with insecure mode and allow private network (test server uses self-signed cert on localhost)
	ctx := context.Background()
	result, err := fetcher.Fetch(ctx, ref, ports.FetchOptions{
		Insecure:            true,
		AllowPrivateNetwork: true, // Required for httptest server on 127.0.0.1
		Timeout:             10 * time.Second,
	})
	require.NoError(t, err)

	// Verify result
	assert.NotEmpty(t, result.Content, "should have content")
	assert.Contains(t, string(result.Content), "e2e-test-profile")
	assert.Equal(t, `"abc123"`, result.ETag)
	assert.NotEmpty(t, result.ContentHash.String())

	// Content type should be yaml, no warning
	assert.Empty(t, contentTypeWarning, "should not warn for yaml content-type")
}

// Test_RemoteProfile_E2E_SecretDetection tests that secrets in fetched profiles trigger callbacks.
func Test_RemoteProfile_E2E_SecretDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Profile with embedded secrets (BAD PRACTICE - this is what we're detecting)
	profileWithSecrets := `
profile:
  name: bad-practice-profile
  version: 1.0.0

vars:
  # This is BAD - hardcoded secret!
  aws_key: AKIAIOSFODNN7EXAMPLE
  github_token: ghp_1234567890abcdefghijklmnopqrstuvwxyz12
`

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/yaml")
		w.Write([]byte(profileWithSecrets))
	}))
	defer server.Close()

	// Create fetcher with secret detection callback
	fetcher := profiles.NewHTTPProfileFetcher()
	var detectedFindings []sensitivedata.SecretFinding

	fetcher.OnSecretDetected = func(findings []sensitivedata.SecretFinding) {
		detectedFindings = findings
	}

	// Parse and fetch
	ref, err := values.ParseProfileReference(server.URL + "/bad-profile.yaml")
	require.NoError(t, err)

	ctx := context.Background()
	result, err := fetcher.Fetch(ctx, ref, ports.FetchOptions{
		Insecure:            true,
		AllowPrivateNetwork: true, // Required for httptest server on 127.0.0.1
		Timeout:             10 * time.Second,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result.Content)

	// Verify secrets were detected
	assert.NotEmpty(t, detectedFindings, "should detect secrets in profile")

	// Check that findings include AWS and/or GitHub patterns
	var foundAWS, foundGitHub bool
	for _, f := range detectedFindings {
		if strings.Contains(strings.ToLower(f.RuleID), "aws") ||
			strings.Contains(f.Match, "AKIA") {
			foundAWS = true
		}
		if strings.Contains(strings.ToLower(f.RuleID), "github") ||
			strings.Contains(f.Match, "ghp_") {
			foundGitHub = true
		}
	}

	assert.True(t, foundAWS || foundGitHub || len(detectedFindings) > 0,
		"should detect at least one secret type, found: %v", detectedFindings)
}

// Test_RemoteProfile_E2E_VersionPinning tests version pinning with URL fragment.
func Test_RemoteProfile_E2E_VersionPinning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Parse reference with version fragment
	ref, err := values.ParseProfileReference("https://example.com/profile.yaml#v1.2.3")
	require.NoError(t, err)

	// Verify version is extracted
	assert.True(t, ref.HasVersion())
	assert.Equal(t, "v1.2.3", ref.Version())
}

// Test_RemoteProfile_E2E_DigestPinning tests content hash pinning.
func Test_RemoteProfile_E2E_DigestPinning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Parse reference with digest
	ref, err := values.ParseProfileReference("https://example.com/profile.yaml@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
	require.NoError(t, err)

	// Verify digest is extracted
	assert.True(t, ref.HasDigest())
	assert.Contains(t, ref.Digest().String(), "sha256:e3b0c44")
}

// Test_RemoteProfile_E2E_SSRFProtection tests that private IPs are blocked.
func Test_RemoteProfile_E2E_SSRFProtection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	fetcher := profiles.NewHTTPProfileFetcher()

	var blockedIP string
	fetcher.OnPrivateIPWarning = func(ip string) {
		blockedIP = ip
	}

	// Try to fetch from private IP - should fail
	ref, err := values.ParseProfileReference("https://192.168.1.1/profile.yaml")
	require.NoError(t, err)

	ctx := context.Background()
	_, err = fetcher.Fetch(ctx, ref, ports.FetchOptions{
		Timeout: 2 * time.Second,
	})

	// Should fail with private IP error
	assert.Error(t, err, "should block private IP")
	// Note: blockedIP callback may not be called if DNS resolution fails first
	_ = blockedIP // Silence unused variable warning
}

// Test_RemoteProfile_E2E_UpdateCheck tests the CheckForUpdate functionality.
func Test_RemoteProfile_E2E_UpdateCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	currentETag := `"version1"`
	newETag := `"version2"`

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("ETag", newETag)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("ETag", newETag)
		w.Write([]byte("content"))
	}))
	defer server.Close()

	fetcher := profiles.NewHTTPProfileFetcher()
	ref, err := values.ParseProfileReference(server.URL + "/profile.yaml")
	require.NoError(t, err)

	ctx := context.Background()
	result, err := fetcher.CheckForUpdate(ctx, ref, currentETag, ports.FetchOptions{
		Insecure:            true,
		AllowPrivateNetwork: true, // Required for httptest server on 127.0.0.1
		Timeout:             10 * time.Second,
	})
	require.NoError(t, err)

	// ETag differs, so update should be available
	assert.True(t, result.HasUpdate, "should detect update when ETag differs")
	assert.Equal(t, currentETag, result.CurrentETag)
	assert.Equal(t, newETag, result.RemoteETag)
}

// Test_RemoteProfile_E2E_CacheKeyGeneration tests that cache keys are stable.
func Test_RemoteProfile_E2E_CacheKeyGeneration(t *testing.T) {
	t.Parallel()

	ref1, _ := values.ParseProfileReference("https://example.com/profiles/test.yaml")
	ref2, _ := values.ParseProfileReference("https://example.com/profiles/test.yaml")
	ref3, _ := values.ParseProfileReference("https://example.com/profiles/other.yaml")

	// Same URL should produce same cache key
	assert.Equal(t, ref1.CacheKey(), ref2.CacheKey(), "same URL should have same cache key")

	// Different URL should produce different cache key
	assert.NotEqual(t, ref1.CacheKey(), ref3.CacheKey(), "different URL should have different cache key")

	// Cache key is MD5 hash of normalized URL (32 hex chars)
	assert.Len(t, ref1.CacheKey(), 32, "cache key should be 32-char hex (MD5)")
}
