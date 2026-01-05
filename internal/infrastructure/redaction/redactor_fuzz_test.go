package redaction

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// FuzzRedactorScrubString fuzzes the redactor for ReDoS and panic conditions
func FuzzRedactorScrubString(f *testing.F) {
	seeds := []string{
		"password=secret",
		"AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE",
		"-----BEGIN PRIVATE KEY-----",
		strings.Repeat("a", 1000),
		"xoxb-123456789012-1234567890123-token",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PANIC on input %q: %v", input, r)
			}
		}()

		// Initialize redactor with default patterns
		r, err := New(Config{
			DisableGitleaks: true, // Disable gitleaks for speed in fuzzing loop, testing regexes primarily
			Patterns: []string{
				`\b((?:AKIA|ABIA|ACCA|ASIA)[0-9A-Z]{16})\b`,
			},
		})
		if err != nil {
			return // Config error, not interesting for fuzzing input
		}

		// ReDoS protection check
		done := make(chan bool, 1)
		go func() {
			_ = r.ScrubString(input)
			done <- true
		}()

		select {
		case <-done:
			// Success
		case <-time.After(1 * time.Second): // 1s timeout for regex processing
			t.Errorf("TIMEOUT (possible ReDoS) on input length %d", len(input))
		}
	})
}

// FuzzRedactorWalker fuzzes recursive redaction of complex structures
func FuzzRedactorWalker(f *testing.F) {
	// Seeds: JSON representing complex objects
	seeds := []string{
		`{"key": "value"}`,
		`{"nested": {"secret": "value"}}`,
		`[{"a": 1}, {"b": 2}]`,
		`{"deep": {"deep": {"deep": "value"}}}`,
	}

	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PANIC: %v", r)
			}
		}()

		// Unmarshal into generic interface
		var input interface{}
		if err := json.Unmarshal(data, &input); err != nil {
			return // Invalid JSON, not interesting
		}

		r, _ := New(Config{DisableGitleaks: true})

		// Walk the structure
		_ = r.Redact(input)
	})
}
