package config

import (
	"bytes"
	"strings"
	"testing"
)

// FuzzYAMLLoading fuzzes YAML parsing for DoS and malformed input
// TARGETS: LoadProfileFromReader() via yaml.Decoder
// EXPECTED FAILURES: Panic on deeply nested YAML, memory exhaustion, invalid UTF-8
func FuzzYAMLLoading(f *testing.F) {
	// Seed corpus with known edge cases
	seeds := []string{
		// Valid profile
		`profile:
  name: test
  version: 1.0.0
controls:
  items: []`,

		// Deeply nested
		strings.Repeat("nested:\n  ", 1000) + "value: 1",

		// Large document
		"controls:\n  items:\n" + strings.Repeat("    - id: test\n", 10000),

		// Invalid UTF-8
		"profile:\n  name: \xff\xfe",

		// Circular reference
		`profile: &anchor
  name: test
  ref: *anchor`,

		// Null bytes
		"profile:\n  name: test\x00null",

		// Empty
		"",

		// Only whitespace
		"   \n\t  \n",

		// Malformed YAML
		"profile:\n  name: test\n    invalid_indent",

		// Very long keys
		strings.Repeat("x", 100000) + ": value",

		// Very long values
		"key: " + strings.Repeat("x", 100000),

		// Unicode edge cases
		"profile:\n  name: \U0001F600\u200B\uFEFF",
	}

	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, yamlData []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PANIC on input (len=%d): %v", len(yamlData), r)
			}
		}()

		reader := bytes.NewReader(yamlData)
		loader := NewProfileLoader()

		// Should handle all inputs gracefully (error or success, no panic)
		_, err := loader.LoadProfileFromReader(reader)
		_ = err // Ignore error, just check for panic
	})
}
