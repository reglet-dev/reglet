// Package redaction handles secret redaction
package redaction

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/viper"
	"github.com/zricethezav/gitleaks/v8/config"
	"github.com/zricethezav/gitleaks/v8/detect"
)

// Redactor handles sanitization of sensitive data.
// All fields are read-only after construction, making it safe for concurrent use.
type Redactor struct {
	patterns []*regexp.Regexp
	paths    []string
	hashMode bool
	salt     string

	// Gitleaks detector for secret detection (222+ patterns)
	// If nil, falls back to regex patterns only
	gitleaksDetector *detect.Detector
}

// Config holds the configuration for the Redactor.
type Config struct {
	// Custom patterns to redact (e.g. "INT-[A-Z0-9]{16}")
	Patterns []string
	// JSON paths to always redact (e.g. "config.password")
	Paths []string
	// If true, replace with hash instead of [REDACTED]
	HashMode bool
	// Salt for hashing (prevents rainbow tables). If empty, hash is deterministic but unsalted.
	Salt string
	// If true, disable gitleaks detector and use only custom patterns
	// Default: false (gitleaks enabled for comprehensive 222+ pattern coverage)
	DisableGitleaks bool
}

// New creates a new Redactor with the given configuration.
func New(cfg Config) (*Redactor, error) {
	r := &Redactor{
		paths:    cfg.Paths,
		hashMode: cfg.HashMode,
		salt:     cfg.Salt,
		patterns: make([]*regexp.Regexp, 0, len(cfg.Patterns)+len(defaultPatterns)),
	}

	// Initialize gitleaks detector (unless disabled)
	if !cfg.DisableGitleaks {
		detector, err := newGitleaksDetector()
		if err != nil {
			// Log warning but don't fail - fall back to regex patterns
			// In production, we might want to log this
			// For now, just continue without gitleaks
		} else {
			r.gitleaksDetector = detector
		}
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

	// Phase 2: Apply custom regex patterns (fallback + user-defined patterns)
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
		// Copy map to avoid mutating original if we want immutability,
		// but strictly speaking we can mutate in place for efficiency.
		// Let's mutate in place for now, but we need to be careful about concurrency if data is shared.
		// Given this is used in single-threaded execution context per control, it should be fine.
		// Actually, map iteration order is random, but keys are stable.
		for k, val := range v {
			nextPath := k
			if currentPath != "" {
				nextPath = currentPath + "." + k
			}
			v[k] = r.walk(val, nextPath)
		}
		return v

	case []interface{}:
		for i, val := range v {
			v[i] = r.walk(val, currentPath) // Lists don't extend the named path? Or use [index]?
			// Usually paths target keys. "items[0].password" is hard to match with simple glob.
			// Let's assume path stays same for array items for now (like "users.password" applies to all users)
			// or we don't track array indices in path.
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
// Format: [hmac:a1b2c3d4e5f6g7h8]
//
// Security notes:
// - Uses HMAC-SHA256 with the configured salt as the key.
// - Truncation to 8 bytes (16 hex chars) prevents rainbow table attacks while allowing correlation.
// - Requires a high-entropy salt for security against offline brute-forcing.
func (r *Redactor) hash(secret string) string {
	mac := hmac.New(sha256.New, []byte(r.salt))
	mac.Write([]byte(secret))
	sum := mac.Sum(nil)

	// Use first 8 bytes (16 hex chars) for correlation
	return fmt.Sprintf("[hmac:%s]", hex.EncodeToString(sum)[:16])
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
