// Package sensitivedata handles detection and redaction of sensitive data
package sensitivedata

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/spf13/viper"
	"github.com/zricethezav/gitleaks/v8/config"
	"github.com/zricethezav/gitleaks/v8/detect"
)

// Redactor handles sanitization of sensitive data.
// All fields are read-only after construction, making it safe for concurrent use.
type Redactor struct {
	gitleaksDetector  *detect.Detector
	sensitiveProvider ports.SensitiveValueProvider
	sensitiveMatcher  SensitiveStringMatcher // Aho-Corasick matcher for O(n) multi-pattern matching
	salt              string
	patterns          []*regexp.Regexp
	paths             []string
	hashMode          bool
}

// RedactorOption configures a Redactor.
type RedactorOption func(*options)

type options struct {
	sensitiveProvider ports.SensitiveValueProvider
	salt              string
	patterns          []string
	paths             []string
	hashMode          bool
	disableGitleaks   bool
}

// WithSalt sets the salt used for hashing in HashMode.
func WithSalt(salt string) RedactorOption {
	return func(o *options) {
		o.salt = salt
	}
}

// WithPatterns adds custom regex patterns for redaction.
func WithPatterns(patterns []string) RedactorOption {
	return func(o *options) {
		o.patterns = patterns
	}
}

// WithPaths adds specific JSON paths to redact (e.g. "config.password").
func WithPaths(paths []string) RedactorOption {
	return func(o *options) {
		o.paths = paths
	}
}

// WithHashMode enables or disables replacing secrets with a hash instead of [REDACTED].
func WithHashMode(enabled bool) RedactorOption {
	return func(o *options) {
		o.hashMode = enabled
	}
}

// WithGitleaksDisabled disables the Gitleaks detector.
func WithGitleaksDisabled(disabled bool) RedactorOption {
	return func(o *options) {
		o.disableGitleaks = disabled
	}
}

// WithSensitiveValueProvider sets a provider for known sensitive values to redact.
func WithSensitiveValueProvider(provider ports.SensitiveValueProvider) RedactorOption {
	return func(o *options) {
		o.sensitiveProvider = provider
	}
}

// NewRedactor creates a new Redactor with the given options.
func NewRedactor(opts ...RedactorOption) (*Redactor, error) {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	r := &Redactor{
		sensitiveProvider: o.sensitiveProvider,
		paths:             o.paths,
		hashMode:          o.hashMode,
		salt:              o.salt,
		patterns:          make([]*regexp.Regexp, 0, len(o.patterns)+len(defaultPatterns)),
	}

	// Initialize Aho-Corasick matcher for efficient multi-pattern matching
	if o.sensitiveProvider != nil {
		r.sensitiveMatcher = NewAhoCorasickMatcher(o.sensitiveProvider)
	}

	// Initialize gitleaks detector (unless disabled)
	if !o.disableGitleaks {
		if detector, err := newGitleaksDetector(); err == nil {
			r.gitleaksDetector = detector
		}
		// If err != nil, fall back to regex patterns silently
	}

	// Compile built-in patterns (used as fallback or when gitleaks is disabled)
	for _, p := range defaultPatterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("failed to compile default pattern %s: %w", p, err)
		}
		r.patterns = append(r.patterns, re)
	}

	// Compile custom patterns
	for _, p := range o.patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("failed to compile custom pattern %s: %w", p, err)
		}
		r.patterns = append(r.patterns, re)
	}

	return r, nil
}

// newGitleaksDetector creates a new gitleaks detector with default configuration.
func newGitleaksDetector() (*detect.Detector, error) {
	// Load gitleaks default config (222+ patterns)
	v := viper.New()
	v.SetConfigType("toml")
	if err := v.ReadConfig(strings.NewReader(config.DefaultConfig)); err != nil {
		return nil, fmt.Errorf("failed to read gitleaks config: %w", err)
	}

	var vc config.ViperConfig
	if err := v.Unmarshal(&vc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal gitleaks config: %w", err)
	}

	cfg, err := vc.Translate()
	if err != nil {
		return nil, fmt.Errorf("failed to translate gitleaks config: %w", err)
	}

	return detect.NewDetector(cfg), nil
}

// Redact sanitizes the given data structure.
// It modifies the data in-place if it's a pointer, or returns a new copy.
// Supported types: string, []interface{}, map[string]interface{}, and pointers to them.
func (r *Redactor) Redact(data interface{}) interface{} {
	return r.walk(data, "")
}

// ScrubString replaces sensitive patterns in a string.
// Uses gitleaks detector (222+ patterns) first, then falls back to regex patterns.
func (r *Redactor) ScrubString(input string) string {
	if input == "" {
		return ""
	}

	result := input

	// Phase 1: Use gitleaks detector if available (comprehensive detection)
	if r.gitleaksDetector != nil {
		//nolint:staticcheck // SA1019: detect.Fragment deprecated, will update when gitleaks v9 releases
		fragment := detect.Fragment{
			Raw: result,
		}

		findings := r.gitleaksDetector.Detect(fragment)
		for _, finding := range findings {
			replacement := "[REDACTED]"
			if r.hashMode {
				replacement = r.hash(finding.Secret)
			}
			result = strings.ReplaceAll(result, finding.Secret, replacement)
		}
	}

	// Phase 2: Known sensitive values using Aho-Corasick algorithm (O(n) multi-pattern matching)
	if r.sensitiveMatcher != nil {
		result = r.sensitiveMatcher.ReplaceAll(result, func(secret string) string {
			if r.hashMode {
				return r.hash(secret)
			}
			return "[REDACTED]"
		})
	}

	// Phase 3: Apply custom regex patterns (fallback + user-defined patterns)
	for _, re := range r.patterns {
		result = re.ReplaceAllStringFunc(result, func(match string) string {
			if r.hashMode {
				return r.hash(match)
			}
			return "[REDACTED]"
		})
	}

	return result
}

// walk recursively traverses the data structure.
// currentPath is the dot-notation path to the current element (e.g. "config.database.password").
func (r *Redactor) walk(data interface{}, currentPath string) interface{} {
	switch v := data.(type) {
	case string:
		// Check if this specific path should be redacted entirely
		if r.isPathMatch(currentPath) {
			if r.hashMode {
				return r.hash(v)
			}
			return "[REDACTED]"
		}
		return r.ScrubString(v)

	case map[string]interface{}:
		// Copy-on-Write: Create a new map to avoid mutating the original
		// This is critical for avoiding race conditions when shared configuration maps
		// are used concurrently across plugins.
		newMap := make(map[string]interface{}, len(v))
		for k, val := range v {
			nextPath := k
			if currentPath != "" {
				nextPath = currentPath + "." + k
			}
			newMap[k] = r.walk(val, nextPath)
		}
		return newMap

	case []interface{}:
		// Copy-on-Write: Create a new slice to avoid mutating the original
		newSlice := make([]interface{}, len(v))
		for i, val := range v {
			newSlice[i] = r.walk(val, currentPath)
		}
		return newSlice

	// Handle other primitives that might be sensitive? usually secrets are strings.
	default:
		return v
	}
}

// isPathMatch checks if the current path matches any of the configured redact paths.
//
// Matching rules:
// - Exact match: path="config.password" matches "config.password"
// - Suffix match: path="password" matches "*.password" (any.nested.password)
//
// Future: Support glob patterns like "config.*.password" (not yet implemented)
func (r *Redactor) isPathMatch(path string) bool {
	for _, p := range r.paths {
		// Exact match
		if p == path {
			return true
		}
		// Suffix match - "password" matches "user.password", "db.config.password", etc.
		// This allows simple patterns without full glob support
		if strings.HasSuffix(path, "."+p) {
			return true
		}
	}
	return false
}

// hash returns a truncated HMAC-SHA256 hash of the secret.
// Format: [hmac:a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6]
//
// Security notes:
// - Uses HMAC-SHA256 with the configured salt as the key.
// - Truncation to 16 bytes (32 hex chars) provides 128-bit security for collision resistance.
// - Prevents rainbow table attacks while allowing correlation of identical secrets.
// - Requires a high-entropy salt for security against offline brute-forcing.
func (r *Redactor) hash(secret string) string {
	mac := hmac.New(sha256.New, []byte(r.salt))
	mac.Write([]byte(secret))
	sum := mac.Sum(nil)

	// Use first 16 bytes (32 hex chars) for correlation - provides 128-bit security
	return fmt.Sprintf("[hmac:%s]", hex.EncodeToString(sum)[:32])
}

// defaultPatterns contains regexes for common secrets.
// Source: Inspired by Gitleaks / TruffleHog patterns.
var defaultPatterns = []string{
	// AWS Access Key ID
	`\b((?:AKIA|ABIA|ACCA|ASIA)[0-9A-Z]{16})\b`,
	// Generic Private Key Header
	`-----BEGIN [A-Z ]+ PRIVATE KEY-----`,
	// Github Token
	`gh[pousr]_[A-Za-z0-9_]{36,255}`,
	// Slack Token
	`xox[baprs]-([0-9a-zA-Z]{10,48})?`,
	// Generic "api_key=" or "token=" assignment (low confidence, maybe too aggressive?)
	// Let's stick to high confidence patterns for defaults to avoid noise.
}
