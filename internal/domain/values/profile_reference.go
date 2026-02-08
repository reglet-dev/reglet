package values

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	hostValues "github.com/reglet-dev/reglet-host-sdk/plugin/values"
)

// ProfileReference is an immutable value object representing a remote profile location.
// Supports HTTPS and OCI schemes with optional version/hash pinning.
//
// URL formats:
//   - https://example.com/profile.yaml
//   - https://example.com/profile.yaml#v1.2.0 (version fragment)
//   - https://example.com/profile.yaml@sha256:abc123 (digest suffix)
//   - oci://ghcr.io/org/profiles/baseline:v1.0.0
//   - oci://ghcr.io/org/profiles/baseline@sha256:abc123
//
// Invariants:
//   - scheme must be "https" or "oci"
//   - host must not be empty
//   - path must not be empty
//   - version and digest are mutually exclusive
//   - rawURL never contains credentials
type ProfileReference struct {
	scheme  string  // "https" or "oci"
	host    string  // "example.com" or "ghcr.io"
	path    string  // "/org/profiles/baseline.yaml"
	version string  // "v1.2.0" (from #fragment)
	digest  *hostValues.Digest // sha256:abc123 (from @suffix)
	rawURL  string  // Original URL for display (credentials stripped)
}

// ParseProfileReference parses a URL string into a ProfileReference.
// Credentials in the URL are automatically stripped.
func ParseProfileReference(rawURL string) (ProfileReference, error) {
	if rawURL == "" {
		return ProfileReference{}, fmt.Errorf("empty URL")
	}

	// Strip credentials before any processing
	sanitizedURL := stripCredentialsFromURL(rawURL)

	// Extract digest suffix before URL parsing (@ is special in URLs)
	baseURL, digestStr := splitDigestSuffix(sanitizedURL)

	// Parse the URL
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return ProfileReference{}, fmt.Errorf("invalid URL: %w", err)
	}

	// Validate scheme
	scheme := strings.ToLower(parsed.Scheme)
	switch scheme {
	case "https", "oci":
		// Valid
	case "http":
		return ProfileReference{}, fmt.Errorf("HTTP URLs are not allowed, use HTTPS or pass --insecure flag")
	case "":
		return ProfileReference{}, fmt.Errorf("missing URL scheme (expected https:// or oci://)")
	default:
		return ProfileReference{}, fmt.Errorf("unsupported URL scheme: %s (expected https or oci)", scheme)
	}

	// Validate host
	if parsed.Host == "" {
		return ProfileReference{}, fmt.Errorf("missing host in URL")
	}

	// Validate path
	path := parsed.Path
	if path == "" {
		return ProfileReference{}, fmt.Errorf("missing path in URL")
	}

	// Extract version from fragment
	version := parsed.Fragment

	// Parse digest if present
	var digest *hostValues.Digest
	if digestStr != "" {
		d, err := hostValues.ParseDigest(digestStr)
		if err != nil {
			return ProfileReference{}, fmt.Errorf("invalid digest in URL: %w", err)
		}
		digest = &d
	}

	// Validate mutual exclusivity of version and digest
	if version != "" && digest != nil {
		return ProfileReference{}, fmt.Errorf("URL cannot have both version fragment and digest suffix")
	}

	return ProfileReference{
		scheme:  scheme,
		host:    parsed.Host,
		path:    path,
		version: version,
		digest:  digest,
		rawURL:  sanitizedURL,
	}, nil
}

// String returns the canonical URL without credentials.
func (r ProfileReference) String() string {
	var sb strings.Builder
	sb.WriteString(r.scheme)
	sb.WriteString("://")
	sb.WriteString(r.host)
	sb.WriteString(r.path)

	if r.digest != nil {
		sb.WriteString("@")
		sb.WriteString(r.digest.String())
	} else if r.version != "" {
		sb.WriteString("#")
		sb.WriteString(r.version)
	}

	return sb.String()
}

// CacheKey returns a deterministic key for cache storage.
// Uses SHA256 of the normalized URL (without version/digest).
func (r ProfileReference) CacheKey() string {
	// Normalize: lowercase scheme and host, no version/digest
	normalized := fmt.Sprintf("%s://%s%s",
		strings.ToLower(r.scheme),
		strings.ToLower(r.host),
		r.path,
	)
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:16]) // Use first 16 bytes (32 hex chars)
}

// Scheme returns the URL scheme ("https" or "oci").
func (r ProfileReference) Scheme() string {
	return r.scheme
}

// Host returns the host portion of the URL.
func (r ProfileReference) Host() string {
	return r.host
}

// Path returns the path portion of the URL.
func (r ProfileReference) Path() string {
	return r.path
}

// Version returns the version from the URL fragment, or empty string.
func (r ProfileReference) Version() string {
	return r.version
}

// Digest returns the digest from the URL suffix, or nil.
func (r ProfileReference) Digest() *hostValues.Digest {
	return r.digest
}

// RawURL returns the original URL (with credentials stripped).
func (r ProfileReference) RawURL() string {
	return r.rawURL
}

// IsOCI returns true if this is an OCI registry reference.
func (r ProfileReference) IsOCI() bool {
	return r.scheme == "oci"
}

// IsHTTPS returns true if this is an HTTPS URL.
func (r ProfileReference) IsHTTPS() bool {
	return r.scheme == "https"
}

// HasVersion returns true if a version was specified.
func (r ProfileReference) HasVersion() bool {
	return r.version != ""
}

// HasDigest returns true if a digest was specified.
func (r ProfileReference) HasDigest() bool {
	return r.digest != nil
}

// WithVersion returns a new ProfileReference with the specified version.
// Clears any existing digest.
func (r ProfileReference) WithVersion(version string) ProfileReference {
	return ProfileReference{
		scheme:  r.scheme,
		host:    r.host,
		path:    r.path,
		version: version,
		digest:  nil,
		rawURL:  r.rawURL,
	}
}

// WithDigest returns a new ProfileReference with the specified digest.
// Clears any existing version.
func (r ProfileReference) WithDigest(digest hostValues.Digest) ProfileReference {
	return ProfileReference{
		scheme:  r.scheme,
		host:    r.host,
		path:    r.path,
		version: "",
		digest:  &digest,
		rawURL:  r.rawURL,
	}
}

// Equals checks equality with another ProfileReference.
func (r ProfileReference) Equals(other ProfileReference) bool {
	if r.scheme != other.scheme ||
		r.host != other.host ||
		r.path != other.path ||
		r.version != other.version {
		return false
	}

	// Compare digests
	if r.digest == nil && other.digest == nil {
		return true
	}
	if r.digest == nil || other.digest == nil {
		return false
	}
	return r.digest.Equals(*other.digest)
}

// IsRemote returns true if this reference points to a remote resource.
// Always true for ProfileReference (as opposed to local file paths).
func (r ProfileReference) IsRemote() bool {
	return true
}

// splitDigestSuffix splits a URL at the @ symbol to extract digest.
// Returns base URL and digest string (empty if no @).
func splitDigestSuffix(rawURL string) (string, string) {
	// Find @ that's not in the authority portion (user:pass@host)
	// We look for @ after :// and after the host
	schemeEnd := strings.Index(rawURL, "://")
	if schemeEnd == -1 {
		return rawURL, ""
	}

	// Find the path start (first / after ://)
	pathStart := strings.Index(rawURL[schemeEnd+3:], "/")
	if pathStart == -1 {
		return rawURL, ""
	}
	pathStart += schemeEnd + 3

	// Look for @ only in the path portion
	pathPortion := rawURL[pathStart:]
	atIndex := strings.LastIndex(pathPortion, "@")
	if atIndex == -1 {
		return rawURL, ""
	}

	// Check if it looks like a digest (contains :)
	digestPart := pathPortion[atIndex+1:]
	if !strings.Contains(digestPart, ":") {
		return rawURL, ""
	}

	return rawURL[:pathStart+atIndex], digestPart
}

// stripCredentialsFromURL removes user:password@ from a URL.
func stripCredentialsFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL // Return as-is if unparseable
	}

	// Clear user info
	parsed.User = nil

	return parsed.String()
}

// IsValidProfileURL checks if a string looks like a remote profile URL.
func IsValidProfileURL(s string) bool {
	return strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "oci://")
}
