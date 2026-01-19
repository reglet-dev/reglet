package values

import (
	"fmt"
	"strings"
)

// TrustedSourcePattern is an immutable value object representing a glob pattern
// for matching trusted profile sources.
//
// Examples:
//   - "https://internal.company.com/*"
//   - "https://profiles.reglet.dev/*"
//   - "oci://ghcr.io/myorg/*"
//
// Invariants:
//   - pattern must be valid glob syntax
//   - pattern must start with "https://" or "oci://"
type TrustedSourcePattern struct {
	pattern string // The original pattern string
}

// ParseTrustedSourcePattern validates and creates a TrustedSourcePattern.
func ParseTrustedSourcePattern(pattern string) (TrustedSourcePattern, error) {
	if pattern == "" {
		return TrustedSourcePattern{}, fmt.Errorf("empty pattern")
	}

	// Validate scheme
	if !strings.HasPrefix(pattern, "https://") && !strings.HasPrefix(pattern, "oci://") {
		return TrustedSourcePattern{}, fmt.Errorf("pattern must start with https:// or oci://, got: %s", pattern)
	}

	return TrustedSourcePattern{
		pattern: pattern,
	}, nil
}

// Matches checks if the given ProfileReference matches this pattern.
func (p TrustedSourcePattern) Matches(ref ProfileReference) bool {
	// Build the URL to match against (without version/digest)
	urlToMatch := fmt.Sprintf("%s://%s%s", ref.Scheme(), ref.Host(), ref.Path())

	return matchURLPattern(p.pattern, urlToMatch)
}

// MatchesURL checks if the given URL string matches this pattern.
// This is a convenience method that avoids parsing a full ProfileReference.
func (p TrustedSourcePattern) MatchesURL(url string) bool {
	// Strip any version fragment or digest suffix for matching
	url = stripVersionAndDigest(url)

	return matchURLPattern(p.pattern, url)
}

// String returns the original pattern string.
func (p TrustedSourcePattern) String() string {
	return p.pattern
}

// Equals checks equality with another TrustedSourcePattern.
func (p TrustedSourcePattern) Equals(other TrustedSourcePattern) bool {
	return p.pattern == other.pattern
}

// matchURLPattern matches a URL against a glob-like pattern.
// Unlike filepath.Match, * matches across path separators for URLs.
// This provides intuitive URL matching behavior.
func matchURLPattern(pattern, url string) bool {
	// Handle trailing wildcard specially (most common case)
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(url, prefix)
	}

	// Handle single trailing wildcard (matches anything)
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(url, prefix)
	}

	// For patterns with wildcards in the middle, use a simple glob approach
	// Split on * and check that all parts appear in order
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		remaining := url
		for i, part := range parts {
			if part == "" {
				continue
			}
			idx := strings.Index(remaining, part)
			if idx == -1 {
				return false
			}
			// First part must be at the start
			if i == 0 && idx != 0 {
				return false
			}
			remaining = remaining[idx+len(part):]
		}
		return true
	}

	// Exact match
	return pattern == url
}

// stripVersionAndDigest removes #version or @digest suffix from a URL.
func stripVersionAndDigest(url string) string {
	// Remove fragment first
	if idx := strings.Index(url, "#"); idx != -1 {
		url = url[:idx]
	}

	// Remove digest suffix (@ in path portion)
	schemeEnd := strings.Index(url, "://")
	if schemeEnd == -1 {
		return url
	}

	pathStart := strings.Index(url[schemeEnd+3:], "/")
	if pathStart == -1 {
		return url
	}
	pathStart += schemeEnd + 3

	pathPortion := url[pathStart:]
	if atIdx := strings.LastIndex(pathPortion, "@"); atIdx != -1 {
		// Check if it looks like a digest
		afterAt := pathPortion[atIdx+1:]
		if strings.Contains(afterAt, ":") {
			url = url[:pathStart+atIdx]
		}
	}

	return url
}

// TrustedSourcePatterns is a collection of patterns for matching.
type TrustedSourcePatterns []TrustedSourcePattern

// ParseTrustedSourcePatterns parses multiple pattern strings.
func ParseTrustedSourcePatterns(patterns []string) (TrustedSourcePatterns, error) {
	result := make(TrustedSourcePatterns, 0, len(patterns))
	for _, p := range patterns {
		pattern, err := ParseTrustedSourcePattern(p)
		if err != nil {
			return nil, fmt.Errorf("pattern %q: %w", p, err)
		}
		result = append(result, pattern)
	}
	return result, nil
}

// Matches returns true if any pattern matches the reference.
func (ps TrustedSourcePatterns) Matches(ref ProfileReference) bool {
	for _, p := range ps {
		if p.Matches(ref) {
			return true
		}
	}
	return false
}

// MatchesURL returns true if any pattern matches the URL.
func (ps TrustedSourcePatterns) MatchesURL(url string) bool {
	for _, p := range ps {
		if p.MatchesURL(url) {
			return true
		}
	}
	return false
}
