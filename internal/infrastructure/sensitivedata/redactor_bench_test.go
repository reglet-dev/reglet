package sensitivedata_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/reglet-dev/reglet/internal/infrastructure/sensitivedata"
)

// BenchmarkScrubStringWithSecrets benchmarks ScrubString with varying numbers of tracked secrets.
// This demonstrates the O(n) Aho-Corasick advantage over O(n*m) naive approach.
func BenchmarkScrubStringWithSecrets(b *testing.B) {
	secretCounts := []int{10, 100, 500, 1000}
	inputSizes := []int{500, 5000}

	for _, secretCount := range secretCounts {
		for _, inputSize := range inputSizes {
			name := fmt.Sprintf("secrets=%d/inputSize=%d", secretCount, inputSize)
			b.Run(name, func(b *testing.B) {
				provider := sensitivedata.NewProvider()

				// Generate secrets
				for i := 0; i < secretCount; i++ {
					provider.Track(fmt.Sprintf("secret-%d-value", i))
				}

				// Create input with a few matches
				var sb strings.Builder
				for i := 0; i < inputSize/50; i++ {
					sb.WriteString("Some normal text with secret-0-value and more text. ")
				}
				input := sb.String()

				redactor, _ := sensitivedata.NewRedactor(
					sensitivedata.WithGitleaksDisabled(true),
					sensitivedata.WithSensitiveValueProvider(provider),
				)

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_ = redactor.ScrubString(input)
				}
			})
		}
	}
}

// BenchmarkScrubStringNoSecrets benchmarks ScrubString without any tracked secrets.
func BenchmarkScrubStringNoSecrets(b *testing.B) {
	input := strings.Repeat("Some normal text without any sensitive data. ", 100)

	redactor, _ := sensitivedata.NewRedactor(
		sensitivedata.WithGitleaksDisabled(true),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = redactor.ScrubString(input)
	}
}

// BenchmarkMatcherReplaceAll directly benchmarks the Aho-Corasick matcher.
func BenchmarkMatcherReplaceAll(b *testing.B) {
	secretCounts := []int{10, 100, 1000}

	for _, secretCount := range secretCounts {
		b.Run(fmt.Sprintf("secrets=%d", secretCount), func(b *testing.B) {
			provider := sensitivedata.NewProvider()

			for i := 0; i < secretCount; i++ {
				provider.Track(fmt.Sprintf("secret-%d-value", i))
			}

			matcher := sensitivedata.NewAhoCorasickMatcher(provider)
			input := strings.Repeat("secret-0-value secret-1-value normal text ", 50)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = matcher.ReplaceAll(input, func(s string) string {
					return "[REDACTED]"
				})
			}
		})
	}
}
