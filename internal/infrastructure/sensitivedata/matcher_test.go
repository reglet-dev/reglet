package sensitivedata_test

import (
	"sync"
	"testing"

	"github.com/reglet-dev/reglet/internal/infrastructure/sensitivedata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAhoCorasickMatcher_BasicReplacement(t *testing.T) {
	provider := sensitivedata.NewProvider()
	provider.Track("secret123")
	provider.Track("password456")

	matcher := sensitivedata.NewAhoCorasickMatcher(provider)

	input := "The secret123 and password456 are here."
	result := matcher.ReplaceAll(input, func(secret string) string {
		return "[REDACTED]"
	})

	assert.Equal(t, "The [REDACTED] and [REDACTED] are here.", result)
}

func TestAhoCorasickMatcher_NoMatches(t *testing.T) {
	provider := sensitivedata.NewProvider()
	provider.Track("secret123")

	matcher := sensitivedata.NewAhoCorasickMatcher(provider)

	input := "Nothing to match here."
	result := matcher.ReplaceAll(input, func(secret string) string {
		return "[REDACTED]"
	})

	assert.Equal(t, input, result)
}

func TestAhoCorasickMatcher_EmptyInput(t *testing.T) {
	provider := sensitivedata.NewProvider()
	provider.Track("secret123")

	matcher := sensitivedata.NewAhoCorasickMatcher(provider)

	result := matcher.ReplaceAll("", func(secret string) string {
		return "[REDACTED]"
	})

	assert.Equal(t, "", result)
}

func TestAhoCorasickMatcher_NoSecrets(t *testing.T) {
	provider := sensitivedata.NewProvider()
	matcher := sensitivedata.NewAhoCorasickMatcher(provider)

	input := "No secrets tracked."
	result := matcher.ReplaceAll(input, func(secret string) string {
		return "[REDACTED]"
	})

	assert.Equal(t, input, result)
}

func TestAhoCorasickMatcher_OverlappingMatches(t *testing.T) {
	provider := sensitivedata.NewProvider()
	provider.Track("abc")
	provider.Track("bcd")

	matcher := sensitivedata.NewAhoCorasickMatcher(provider)

	// "abcd" contains both "abc" and "bcd" overlapping
	// First match "abc" at pos 0 should be replaced, "bcd" at pos 1 is overlapping and skipped
	input := "abcd"
	result := matcher.ReplaceAll(input, func(secret string) string {
		return "[X]"
	})

	// After replacing "abc" at 0, we have "[X]d", "bcd" was overlapping
	assert.Equal(t, "[X]d", result)
}

func TestAhoCorasickMatcher_MultipleOccurrences(t *testing.T) {
	provider := sensitivedata.NewProvider()
	provider.Track("secret")

	matcher := sensitivedata.NewAhoCorasickMatcher(provider)

	input := "secret secret secret"
	result := matcher.ReplaceAll(input, func(secret string) string {
		return "[REDACTED]"
	})

	assert.Equal(t, "[REDACTED] [REDACTED] [REDACTED]", result)
}

func TestAhoCorasickMatcher_LazyRebuild(t *testing.T) {
	provider := sensitivedata.NewProvider()
	matcher := sensitivedata.NewAhoCorasickMatcher(provider)

	// Initially no secrets
	input := "Find mysecret here."
	result := matcher.ReplaceAll(input, func(secret string) string {
		return "[X]"
	})
	assert.Equal(t, input, result, "Should not match before tracking")

	// Track a secret - trie should rebuild
	provider.Track("mysecret")

	result = matcher.ReplaceAll(input, func(secret string) string {
		return "[X]"
	})
	assert.Equal(t, "Find [X] here.", result, "Should match after tracking")
}

func TestAhoCorasickMatcher_ConcurrentAccess(t *testing.T) {
	provider := sensitivedata.NewProvider()
	provider.Track("secret1")
	provider.Track("secret2")

	matcher := sensitivedata.NewAhoCorasickMatcher(provider)

	var wg sync.WaitGroup
	workers := 10
	iterations := 100

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				result := matcher.ReplaceAll("secret1 and secret2", func(s string) string {
					return "[X]"
				})
				if result != "[X] and [X]" {
					t.Errorf("Unexpected result: %s", result)
				}
			}
		}()
	}

	wg.Wait()
}

func TestAhoCorasickMatcher_CustomReplacement(t *testing.T) {
	provider := sensitivedata.NewProvider()
	provider.Track("password")
	provider.Track("token")

	matcher := sensitivedata.NewAhoCorasickMatcher(provider)

	// Custom replacement function that uses the secret itself
	input := "My password and token"
	result := matcher.ReplaceAll(input, func(secret string) string {
		return "[HIDDEN:" + secret[:2] + "***]"
	})

	assert.Equal(t, "My [HIDDEN:pa***] and [HIDDEN:to***]", result)
}

// Integration test ensuring matcher works correctly with Redactor
func TestRedactor_Phase2WithAhoCorasick(t *testing.T) {
	provider := sensitivedata.NewProvider()
	provider.Track("api-key-12345")
	provider.Track("db-password-xyz")

	redactor, err := sensitivedata.NewRedactor(
		sensitivedata.WithGitleaksDisabled(true),
		sensitivedata.WithSensitiveValueProvider(provider),
	)
	require.NoError(t, err)

	input := "Config: api-key-12345, database: db-password-xyz"
	result := redactor.ScrubString(input)

	assert.Equal(t, "Config: [REDACTED], database: [REDACTED]", result)
}
