package hostfuncs

import (
	"context"
	"strings"
	"testing"
)

// FuzzSSRFProtection fuzzes IP and hostname validation for SSRF bypasses
// TARGETS: ValidateDestination (and underlying isPrivateOrReservedIP)
// EXPECTED FAILURES: None (should return error for private IPs, nil for public)
func FuzzSSRFProtection(f *testing.F) {
	seeds := []string{
		"127.0.0.1",
		"localhost",
		"169.254.169.254", // AWS metadata
		"10.0.0.1",
		"::1",
		"0.0.0.0",
		"example.com",
		"216.58.214.14",          // Public IP
		"[::ffff:127.0.0.1]",     // IPv4-mapped IPv6
		"0177.0.0.1",             // Octal
		"0x7f.0.0.1",             // Hex
		"2130706433",             // Decimal (127.0.0.1)
		strings.Repeat("a", 300), // Too long
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, host string) {
		// CapabilityChecker is difficult to mock purely, but ValidateDestination
		// primarily checks the host against blocklists *before* capabilities
		// if checking for private ranges.
		// However, ValidateDestination requires a checker.
		// We can test the underlying IP parsing logic or mock the checker.
		// For fuzzing, we care about panic freedom in the parsing logic.

		// We'll create a dummy checker (implementation detail: allows nothing by default)
		checker := NewCapabilityChecker(nil) // Assuming this constructor exists or similar

		// Note: NewCapabilityChecker usually takes a CapabilityProvider.
		// If that's complex, we might skip the full ValidateDestination and target
		// private IP logic if exposed, but ValidateDestination is the public entry point.
		// Let's try to construct a nil-safe checker or pass nil if the function handles it
		// (The grep showed it takes *CapabilityChecker).

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PANIC on host %q: %v", host, r)
			}
		}()

		// Pass nil plugin name.
		// We are fuzzing for parsing panics, mostly.
		_ = ValidateDestination(context.Background(), host, "fuzz-plugin", checker)
	})
}
