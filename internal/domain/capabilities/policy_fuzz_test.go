package capabilities

import (
	"testing"
)

// FuzzNetworkPatternMatching fuzzes port range parsing for integer overflow and off-by-one errors
// TARGETS: matchNetworkPattern() and matchPortRange() functions
// EXPECTED FAILURES: Integer overflow (65536+), negative ports, malformed ranges
func FuzzNetworkPatternMatching(f *testing.F) {
	// Seed corpus with known edge cases
	seeds := []string{
		"outbound:80",               // Single port
		"outbound:80,443",           // Multiple ports
		"outbound:8000-9000",        // Range
		"outbound:65535",            // Max valid port
		"outbound:0",                // Min port (invalid)
		"outbound:65536",            // Overflow
		"outbound:8000-8000",        // Same port range
		"outbound:-1",               // Negative port
		"outbound:80,443,8000-9000", // Combined
		"outbound:",                 // Empty port
		"outbound:abc",              // Non-numeric
		"outbound:8000-7000",        // Reverse range
		"outbound:999999",           // Large number
		"*",                         // Wildcard
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, pattern string) {
		// Should never panic, always return bool + error
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PANIC on input %q: %v", pattern, r)
			}
		}()

		// Test pattern matching - should handle all inputs gracefully
		cap := Capability{Kind: "network", Pattern: pattern}
		requestCap := Capability{Kind: "network", Pattern: "outbound:443"}

		_ = matchNetworkPattern(requestCap.Pattern, cap.Pattern)
		// No panic = success
	})
}

// FuzzFilesystemPatternMatching fuzzes path handling for traversal and symlink issues
func FuzzFilesystemPatternMatching(f *testing.F) {
	seeds := []string{
		"read:/etc/passwd",
		"read:/etc/../etc/passwd",
		"read:/../../../etc/passwd",
		"read:/tmp/../etc/passwd",
		"read://double/slash",
		"read:/path\x00null",
		"read:/very/long/" + string(make([]byte, 4096)),
		"read:/etc/**",
		"read:**",
		"read:../relative",
		"write:/home/user/file",
		"read:",
		":",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, pattern string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PANIC on input %q: %v", pattern, r)
			}
		}()

		cap := Capability{Kind: "fs", Pattern: pattern}
		requestCap := Capability{Kind: "fs", Pattern: "read:/etc/passwd"}

		_ = matchFilesystemPattern(requestCap.Pattern, cap.Pattern)
	})
}

// FuzzExecPatternMatching fuzzes executable and directory wildcard matching
func FuzzExecPatternMatching(f *testing.F) {
	seeds := []string{
		"/usr/bin/ls",
		"/bin/*",
		"/usr/bin/../bin/sh",
		"python",
		"python3.11",
		"/usr/bin/python\x00",
		"sh -c 'malicious'",
		"**",
		"*",
		"/bin/",
		"",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, pattern string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PANIC on input %q: %v", pattern, r)
			}
		}()

		cap := Capability{Kind: "exec", Pattern: pattern}
		requestCap := Capability{Kind: "exec", Pattern: "/usr/bin/ls"}

		_ = matchExecPattern(requestCap.Pattern, cap.Pattern)
	})
}

// FuzzEnvironmentPatternMatching fuzzes wildcard and prefix matching
func FuzzEnvironmentPatternMatching(f *testing.F) {
	seeds := []string{
		"AWS_*",
		"AWS_ACCESS_KEY_ID",
		"*",
		"VAR_",
		"",
		"VAR\x00NULL",
		string(make([]byte, 10000)), // Very long
		"**",
		"AWS_*_*",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, pattern string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PANIC on input %q: %v", pattern, r)
			}
		}()

		_ = MatchEnvironmentPattern("AWS_ACCESS_KEY_ID", pattern)
	})
}
