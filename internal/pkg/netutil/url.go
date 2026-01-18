package netutil

import (
	"net/url"
	"strings"
)

// StripCredentials removes user:password@ from a URL for safe logging.
// Returns the original string if the URL cannot be parsed.
func StripCredentials(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	// Clear user info
	parsed.User = nil

	return parsed.String()
}

// HasCredentials returns true if the URL contains credentials.
func HasCredentials(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return parsed.User != nil
}

// NormalizeURL returns a normalized form of the URL suitable for cache keys.
// It lowercases the scheme and host, removes default ports, and strips credentials.
func NormalizeURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	// Clear credentials
	parsed.User = nil

	// Lowercase scheme and host
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)

	// Remove default ports
	host := parsed.Hostname()
	port := parsed.Port()
	if (parsed.Scheme == "https" && port == "443") ||
		(parsed.Scheme == "http" && port == "80") {
		parsed.Host = host
	}

	// Remove trailing slash from path (except for root)
	if parsed.Path != "/" && strings.HasSuffix(parsed.Path, "/") {
		parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	}

	// Sort query parameters for consistent ordering
	if parsed.RawQuery != "" {
		values := parsed.Query()
		parsed.RawQuery = values.Encode() // Encodes in sorted order
	}

	return parsed.String()
}

// ExtractHost returns just the host:port from a URL.
func ExtractHost(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return parsed.Host
}

// IsHTTPS returns true if the URL uses the HTTPS scheme.
func IsHTTPS(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.ToLower(parsed.Scheme) == "https"
}

// IsOCI returns true if the URL uses the OCI scheme.
func IsOCI(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.ToLower(parsed.Scheme) == "oci"
}
