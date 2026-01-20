// Package sensitivedata handles detection and redaction of sensitive data
package sensitivedata

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
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
		detector, err := newGitleaksDetector()
		if err != nil {
			// TODO: instantiate logger WithLogger.
			slog.Warn("sensitivedata: failed to initialize gitleaks detector, falling back to regex patterns", "error", err)
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

// SecretFinding represents a detected secret in content.
type SecretFinding struct {
	RuleID      string // Identifier for the detection rule (e.g., "aws-access-key")
	Description string // Human-readable description of the secret type
	Match       string // The matched pattern (redacted for logging safety)
	StartLine   int    // 1-indexed line number where secret starts
	EndLine     int    // 1-indexed line number where secret ends
}

// DetectSecrets scans content for potential secrets without modifying it.
// Returns a list of findings. This is useful for warning users about
// hardcoded secrets in fetched remote profiles (Constitution II: Credential Hygiene).
func (r *Redactor) DetectSecrets(content string) []SecretFinding {
	if content == "" {
		return nil
	}

	var findings []SecretFinding

	// Use gitleaks detector if available
	if r.gitleaksDetector != nil {
		//nolint:staticcheck // SA1019: detect.Fragment deprecated, will update when gitleaks v9 releases
		fragment := detect.Fragment{
			Raw: content,
		}

		gitleaksFindings := r.gitleaksDetector.Detect(fragment)
		for _, f := range gitleaksFindings {
			findings = append(findings, SecretFinding{
				RuleID:      f.RuleID,
				Description: f.Description,
				StartLine:   f.StartLine,
				EndLine:     f.EndLine,
				Match:       redactMatch(f.Secret),
			})
		}
	}

	// Also check regex patterns
	lines := strings.Split(content, "\n")
	for lineNum, line := range lines {
		for _, re := range r.patterns {
			if matches := re.FindAllString(line, -1); len(matches) > 0 {
				for _, m := range matches {
					findings = append(findings, SecretFinding{
						RuleID:      "regex-pattern",
						Description: "Matched built-in pattern",
						StartLine:   lineNum + 1,
						EndLine:     lineNum + 1,
						Match:       redactMatch(m),
					})
				}
			}
		}
	}

	return findings
}

// redactMatch partially redacts a match for safe logging.
// Shows first 4 and last 2 chars, redacts the middle.
func redactMatch(s string) string {
	if len(s) <= 8 {
		return "[REDACTED]"
	}
	return s[:4] + "..." + s[len(s)-2:]
}

// Redact sanitizes the given data structure.
// It modifies the data in-place if it's a pointer, or returns a new copy.
// Supported types: string, []interface{}, map[string]interface{}, and pointers to them.
func (r *Redactor) Redact(data interface{}) interface{} {
	return r.walk(data, "")
}

// ScrubString replaces sensitive patterns in a string.
//
// Uses a single-pass collection strategy to avoid multi-pass mutation vulnerabilities:
// 1. Collect all matches from: gitleaks, known sensitive values, and regex patterns
// 2. Deduplicate overlapping matches (prefer longer/more specific matches)
// 3. Apply all replacements in a single pass
func (r *Redactor) ScrubString(input string) string {
	if input == "" {
		return ""
	}

	var allMatches []Match

	// Phase 1: Collect matches from gitleaks detector
	if r.gitleaksDetector != nil {
		//nolint:staticcheck // SA1019: detect.Fragment deprecated, will update when gitleaks v9 releases
		fragment := detect.Fragment{
			Raw: input,
		}

		findings := r.gitleaksDetector.Detect(fragment)
		for _, finding := range findings {
			// Find the position of this secret in the input
			start := strings.Index(input, finding.Secret)
			if start >= 0 {
				allMatches = append(allMatches, Match{
					Start:  start,
					End:    start + len(finding.Secret),
					Secret: finding.Secret,
				})
			}
		}
	}

	// Phase 2: Collect matches from known sensitive values (Aho-Corasick)
	if r.sensitiveMatcher != nil {
		sensitiveMatches := r.sensitiveMatcher.FindAll(input)
		allMatches = append(allMatches, sensitiveMatches...)
	}

	// Phase 3: Collect matches from regex patterns
	for _, re := range r.patterns {
		locs := re.FindAllStringIndex(input, -1)
		for _, loc := range locs {
			allMatches = append(allMatches, Match{
				Start:  loc[0],
				End:    loc[1],
				Secret: input[loc[0]:loc[1]],
			})
		}
	}

	// Deduplicate and apply replacements
	if len(allMatches) == 0 {
		return input
	}

	deduped := SortAndDeduplicateMatches(allMatches)
	return ApplyReplacements(input, deduped, func(secret string) string {
		if r.hashMode {
			return r.hash(secret)
		}
		return "[REDACTED]"
	})
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
	key := []byte(r.salt)
	prefix := "[hmac:%s]"
	if len(key) == 0 {
		// Fallback to a non-empty key to avoid weakening HMAC security when salt is misconfigured.
		// The different prefix makes it clear that the salt configuration is invalid.
		key = []byte("default-hmac-key")
		prefix = "[hmac-unconfigured:%s]"
	}

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(secret))
	sum := mac.Sum(nil)

	// Use first 16 bytes (32 hex chars) for correlation - provides 128-bit security
	return fmt.Sprintf("%s%s", prefix, hex.EncodeToString(sum)[:32])
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
