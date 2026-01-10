package config

import (
	"strings"
	"testing"
	"time"
)

// FuzzVariableSubstitution fuzzes template substitution for ReDoS and injection
// TARGETS: substituteInString(), lookupVar() via regex matching
// EXPECTED FAILURES: ReDoS timeout, stack overflow on deep nesting
func FuzzVariableSubstitution(f *testing.F) {
	seeds := []string{
		// Valid
		"{{.vars.key}}",
		"prefix {{.vars.key}} suffix",

		// Malformed
		"{{.vars",
		"}}",
		"{{.vars.}}",
		"{{.vars..key}}",
		"{{ .vars.key }}",

		// Nested
		"{{.vars.{{.vars.nested}}}}",

		// Special chars
		"{{.vars.key\x00}}",
		"{{.vars.key\n}}",

		// Very long
		"{{.vars." + string(make([]byte, 10000)) + "}}",

		// ReDoS attempt
		"{{" + strings.Repeat("(a+)+", 100) + "}}",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, template string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PANIC on input %q: %v", template, r)
			}
		}()

		// Timeout protection for ReDoS
		done := make(chan bool, 1)
		go func() {
			vars := map[string]interface{}{
				"key":    "value",
				"nested": "test",
			}

			// Call private function directly since we are in the same package
			sub := NewVariableSubstitutor(nil)
			_, _ = sub.substituteInString(template, vars)
			done <- true
		}()

		select {
		case <-done:
			// Success
		case <-time.After(5 * time.Second):
			t.Errorf("TIMEOUT (possible ReDoS) on input %q", template)
		}
	})
}
