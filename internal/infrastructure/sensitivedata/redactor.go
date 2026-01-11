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
	salt              string
	patterns          []*regexp.Regexp
	paths             []string
	hashMode          bool
}

// Config holds the configuration for the Redactor.
type Config struct {
	Salt            string
	Patterns        []string
	Paths           []string
	HashMode        bool
	DisableGitleaks bool
}

// New creates a new Redactor with the given configuration.
func New(cfg Config) (*Redactor, error) {
	return NewWithProvider(cfg, nil)
}

// NewWithProvider creates a new Redactor with the given configuration and provider.
func NewWithProvider(cfg Config, provider ports.SensitiveValueProvider) (*Redactor, error) {
	r := &Redactor{
		sensitiveProvider: provider,
		paths:             cfg.Paths,
		hashMode:          cfg.HashMode,
		salt:              cfg.Salt,
		patterns:          make([]*regexp.Regexp, 0, len(cfg.Patterns)+len(defaultPatterns)),
	}

	// Initialize gitleaks detector (unless disabled)
	if !cfg.DisableGitleaks {
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
	for _, p := range cfg.Patterns {
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

	// Phase 2: Known sensitive values (NEW)
	if r.sensitiveProvider != nil {
		secrets := r.sensitiveProvider.AllValues()
		// Build a single combined regexp to match any known secret in one pass.
		patternParts := make([]string, 0, len(secrets))
		for _, secret := range secrets {
			if secret == "" {
				continue
			}
			patternParts = append(patternParts, regexp.QuoteMeta(secret))
		}
		if len(patternParts) > 0 {
			combinedPattern := strings.Join(patternParts, "|")
			re := regexp.MustCompile(combinedPattern)
			result = re.ReplaceAllStringFunc(result, func(match string) string {
				if r.hashMode {
					return r.hash(match)
				}
				return "[REDACTED]"
			})
		}
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
		for i, val := range v {
			// Arrays intentionally do not extend the path with indices; all elements share the same path.
			// This means a path like "users.password" applies to every element in the "users" list.
			// Index-specific paths such as "users[0].password" are not supported by design.
			v[i] = r.walk(val, currentPath)
		}
		return v

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
	key := []byte(r.salt)
	prefix := "[hmac:%s]"
	if len(key) == 0 {
		// Fallback to a non-empty key to avoid weakening HMAC security when salt is misconfigured.
		// The different prefix makes it clear that the salt configuration is invalid.
		key = []byte("default-hmac-key")
		prefix = "[hmac-misconfigured:%s]"
	}

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(secret))
	sum := mac.Sum(nil)

	// Use first 16 bytes (32 hex chars) for correlation - provides 128-bit security
	return fmt.Sprintf(prefix, hex.EncodeToString(sum)[:32])
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
