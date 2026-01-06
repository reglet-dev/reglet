package hostfuncs

import (
	"net"
	"strings"
	"testing"
)

// FuzzSSRFProtection fuzzes IP parsing and validation for SSRF bypasses.
// TARGETS: IsPrivateOrReservedIP (the core IP validation logic)
//
// This test focuses on the pure IP parsing/validation function rather than
// ValidateDestination to avoid making real DNS lookups during fuzzing.
// DNS operations are slow and can cause test hangs when fuzzing at high volume.
//
// EXPECTED BEHAVIOR: Should never panic; returns true for private/reserved IPs,
// false for public IPs, and gracefully handles unparseable input.
func FuzzSSRFProtection(f *testing.F) {
	seeds := []string{
		// Standard IPs
		"127.0.0.1",
		"169.254.169.254", // AWS metadata
		"10.0.0.1",
		"192.168.1.1",
		"172.16.0.1",
		"8.8.8.8",
		"1.1.1.1",
		"0.0.0.0",

		// IPv6
		"::1",
		"::ffff:127.0.0.1", // IPv4-mapped IPv6 (SSRF bypass vector)
		"::ffff:169.254.169.254",
		"::ffff:8.8.8.8",
		"fc00::1",
		"fe80::1",
		"2001:4860:4860::8888",

		// Known SSRF bypass attempts
		"[::ffff:127.0.0.1]",
		"0177.0.0.1", // Octal
		"0x7f.0.0.1", // Hex
		"2130706433", // Decimal (127.0.0.1)
		"0x7f000001", // Full hex
		"127.1",      // Short form
		"127.0.1",    // Another short form
		"0",          // Zero
		"0.0.0.0.0",  // Too many octets
		"255.255.255.255",

		// Malformed
		"",
		strings.Repeat("a", 300),
		"not-an-ip",
		"127.0.0.1.malicious.com",
		"127.0.0.1:8080",
		"[::1]:8080",
		":::::::",
		"1.2.3.4.5.6.7.8",
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

		// Parse the input as an IP - this handles malformed input gracefully
		ip := net.ParseIP(input)
		if ip == nil {
			// Not a valid IP, nothing to validate
			return
		}

		// The core function we're fuzzing - should never panic
		_ = IsPrivateOrReservedIP(ip)
	})
}
