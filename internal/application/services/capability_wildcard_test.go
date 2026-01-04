package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/whiskeyjimbo/reglet/internal/domain/capabilities"
)

// TestIsBroadCapability_ExecWildcards verifies that exec wildcard patterns are detected as broad.
// This prevents undermining the principle of least privilege.
func TestIsBroadCapability_ExecWildcards(t *testing.T) {
	tests := []struct {
		name       string
		capability capabilities.Capability
		isBroad    bool
	}{
		{
			name:       "exec:** is overly broad (arbitrary command execution)",
			capability: capabilities.Capability{Kind: "exec", Pattern: "**"},
			isBroad:    true,
		},
		{
			name:       "exec:* is overly broad",
			capability: capabilities.Capability{Kind: "exec", Pattern: "*"},
			isBroad:    true,
		},
		{
			name:       "specific binary is not broad",
			capability: capabilities.Capability{Kind: "exec", Pattern: "/usr/bin/ls"},
			isBroad:    false,
		},
		{
			name:       "directory wildcard /bin/* is not broad (limited scope)",
			capability: capabilities.Capability{Kind: "exec", Pattern: "/bin/*"},
			isBroad:    false,
		},
		{
			name:       "shells are broad (allows arbitrary commands)",
			capability: capabilities.Capability{Kind: "exec", Pattern: "/bin/sh"},
			isBroad:    true,
		},
		{
			name:       "interpreters are broad (allows code execution via -c)",
			capability: capabilities.Capability{Kind: "exec", Pattern: "python"},
			isBroad:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.capability.IsBroad() // Use domain method
			assert.Equal(t, tt.isBroad, result,
				"Capability %s should %s be detected as broad",
				tt.capability.String(),
				map[bool]string{true: "", false: "NOT"}[tt.isBroad])
		})
	}
}

// TestIsBroadCapability_FilesystemWildcards verifies that fs wildcard patterns are detected.
func TestIsBroadCapability_FilesystemWildcards(t *testing.T) {
	tests := []struct {
		name       string
		capability capabilities.Capability
		isBroad    bool
	}{
		{
			name:       "fs:read:** is overly broad",
			capability: capabilities.Capability{Kind: "fs", Pattern: "read:**"},
			isBroad:    true,
		},
		{
			name:       "fs:write:** is overly broad",
			capability: capabilities.Capability{Kind: "fs", Pattern: "write:**"},
			isBroad:    true,
		},
		{
			name:       "root filesystem is broad",
			capability: capabilities.Capability{Kind: "fs", Pattern: "/**"},
			isBroad:    true,
		},
		{
			name:       "specific file is not broad",
			capability: capabilities.Capability{Kind: "fs", Pattern: "read:/etc/passwd"},
			isBroad:    false,
		},
		{
			name:       "specific directory tree is not broad",
			capability: capabilities.Capability{Kind: "fs", Pattern: "read:/var/log/**"},
			isBroad:    false,
		},
		{
			name:       "/etc/** is broad (sensitive system config)",
			capability: capabilities.Capability{Kind: "fs", Pattern: "read:/etc/**"},
			isBroad:    true,
		},
		{
			name:       "/home/** is broad (all user data)",
			capability: capabilities.Capability{Kind: "fs", Pattern: "read:/home/**"},
			isBroad:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.capability.IsBroad() // Use domain method
			assert.Equal(t, tt.isBroad, result,
				"Capability %s should %s be detected as broad",
				tt.capability.String(),
				map[bool]string{true: "", false: "NOT"}[tt.isBroad])
		})
	}
}

// TestIsBroadCapability_EnvironmentWildcards verifies env wildcard detection.
func TestIsBroadCapability_EnvironmentWildcards(t *testing.T) {
	tests := []struct {
		name       string
		capability capabilities.Capability
		isBroad    bool
	}{
		{
			name:       "env:* is overly broad (all environment variables)",
			capability: capabilities.Capability{Kind: "env", Pattern: "*"},
			isBroad:    true,
		},
		{
			name:       "AWS_* is broad (all AWS credentials)",
			capability: capabilities.Capability{Kind: "env", Pattern: "AWS_*"},
			isBroad:    true,
		},
		{
			name:       "AZURE_* is broad",
			capability: capabilities.Capability{Kind: "env", Pattern: "AZURE_*"},
			isBroad:    true,
		},
		{
			name:       "GCP_* is broad",
			capability: capabilities.Capability{Kind: "env", Pattern: "GCP_*"},
			isBroad:    true,
		},
		{
			name:       "specific variable is not broad",
			capability: capabilities.Capability{Kind: "env", Pattern: "AWS_REGION"},
			isBroad:    false,
		},
		{
			name:       "custom prefix is not broad",
			capability: capabilities.Capability{Kind: "env", Pattern: "MY_APP_*"},
			isBroad:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.capability.IsBroad() // Use domain method
			assert.Equal(t, tt.isBroad, result,
				"Capability %s should %s be detected as broad",
				tt.capability.String(),
				map[bool]string{true: "", false: "NOT"}[tt.isBroad])
		})
	}
}

// TestIsBroadCapability_NetworkWildcards verifies network wildcard detection.
func TestIsBroadCapability_NetworkWildcards(t *testing.T) {
	tests := []struct {
		name       string
		capability capabilities.Capability
		isBroad    bool
	}{
		{
			name:       "network:* is overly broad",
			capability: capabilities.Capability{Kind: "network", Pattern: "*"},
			isBroad:    true,
		},
		{
			name:       "network:outbound:* is overly broad (any port)",
			capability: capabilities.Capability{Kind: "network", Pattern: "outbound:*"},
			isBroad:    true,
		},
		{
			name:       "specific port is not broad",
			capability: capabilities.Capability{Kind: "network", Pattern: "outbound:443"},
			isBroad:    false,
		},
		{
			name:       "multiple ports is not broad",
			capability: capabilities.Capability{Kind: "network", Pattern: "outbound:80,443"},
			isBroad:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.capability.IsBroad() // Use domain method
			assert.Equal(t, tt.isBroad, result,
				"Capability %s should %s be detected as broad",
				tt.capability.String(),
				map[bool]string{true: "", false: "NOT"}[tt.isBroad])
		})
	}
}

// TestIsBroadCapability_VersionedInterpreters verifies that versioned interpreters are detected as broad.
// This prevents bypass attacks using python3.11 instead of python3, node18 instead of node, etc.
func TestIsBroadCapability_VersionedInterpreters(t *testing.T) {
	tests := []struct {
		name       string
		capability capabilities.Capability
		isBroad    bool
	}{
		// Python versions - all should be detected as broad
		{
			name:       "python (base) is broad",
			capability: capabilities.Capability{Kind: "exec", Pattern: "python"},
			isBroad:    true,
		},
		{
			name:       "python3 is broad",
			capability: capabilities.Capability{Kind: "exec", Pattern: "python3"},
			isBroad:    true,
		},
		{
			name:       "python3.11 is broad (versioned)",
			capability: capabilities.Capability{Kind: "exec", Pattern: "python3.11"},
			isBroad:    true,
		},
		{
			name:       "python3.12 is broad (future version)",
			capability: capabilities.Capability{Kind: "exec", Pattern: "python3.12"},
			isBroad:    true,
		},
		{
			name:       "python2.7 is broad (legacy version)",
			capability: capabilities.Capability{Kind: "exec", Pattern: "python2.7"},
			isBroad:    true,
		},
		{
			name:       "python:* is broad (explicit wildcard)",
			capability: capabilities.Capability{Kind: "exec", Pattern: "python:*"},
			isBroad:    true,
		},

		// Node.js versions
		{
			name:       "node is broad",
			capability: capabilities.Capability{Kind: "exec", Pattern: "node"},
			isBroad:    true,
		},
		{
			name:       "node18 is broad (versioned)",
			capability: capabilities.Capability{Kind: "exec", Pattern: "node18"},
			isBroad:    true,
		},
		{
			name:       "node20 is broad (versioned)",
			capability: capabilities.Capability{Kind: "exec", Pattern: "node20"},
			isBroad:    true,
		},
		{
			name:       "nodejs is broad (alias)",
			capability: capabilities.Capability{Kind: "exec", Pattern: "nodejs"},
			isBroad:    true,
		},

		// PHP versions
		{
			name:       "php is broad",
			capability: capabilities.Capability{Kind: "exec", Pattern: "php"},
			isBroad:    true,
		},
		{
			name:       "php7 is broad (versioned)",
			capability: capabilities.Capability{Kind: "exec", Pattern: "php7"},
			isBroad:    true,
		},
		{
			name:       "php8 is broad (versioned)",
			capability: capabilities.Capability{Kind: "exec", Pattern: "php8"},
			isBroad:    true,
		},

		// Lua versions
		{
			name:       "lua is broad",
			capability: capabilities.Capability{Kind: "exec", Pattern: "lua"},
			isBroad:    true,
		},
		{
			name:       "lua5.4 is broad (versioned)",
			capability: capabilities.Capability{Kind: "exec", Pattern: "lua5.4"},
			isBroad:    true,
		},
		{
			name:       "lua5.1 is broad (versioned)",
			capability: capabilities.Capability{Kind: "exec", Pattern: "lua5.1"},
			isBroad:    true,
		},

		// Ruby versions
		{
			name:       "ruby is broad",
			capability: capabilities.Capability{Kind: "exec", Pattern: "ruby"},
			isBroad:    true,
		},
		{
			name:       "ruby3.2 is broad (versioned)",
			capability: capabilities.Capability{Kind: "exec", Pattern: "ruby3.2"},
			isBroad:    true,
		},
		{
			name:       "irb is broad (interactive ruby)",
			capability: capabilities.Capability{Kind: "exec", Pattern: "irb"},
			isBroad:    true,
		},

		// Perl versions
		{
			name:       "perl is broad",
			capability: capabilities.Capability{Kind: "exec", Pattern: "perl"},
			isBroad:    true,
		},
		{
			name:       "perl5 is broad (versioned)",
			capability: capabilities.Capability{Kind: "exec", Pattern: "perl5"},
			isBroad:    true,
		},

		// Tcl family (was missing in original implementation)
		{
			name:       "tclsh is broad",
			capability: capabilities.Capability{Kind: "exec", Pattern: "tclsh"},
			isBroad:    true,
		},
		{
			name:       "wish is broad (Tcl/Tk)",
			capability: capabilities.Capability{Kind: "exec", Pattern: "wish"},
			isBroad:    true,
		},
		{
			name:       "expect is broad (Tcl-based)",
			capability: capabilities.Capability{Kind: "exec", Pattern: "expect"},
			isBroad:    true,
		},

		// AWK variants
		{
			name:       "awk is broad",
			capability: capabilities.Capability{Kind: "exec", Pattern: "awk"},
			isBroad:    true,
		},
		{
			name:       "gawk is broad",
			capability: capabilities.Capability{Kind: "exec", Pattern: "gawk"},
			isBroad:    true,
		},
		{
			name:       "mawk is broad",
			capability: capabilities.Capability{Kind: "exec", Pattern: "mawk"},
			isBroad:    true,
		},
		{
			name:       "nawk is broad",
			capability: capabilities.Capability{Kind: "exec", Pattern: "nawk"},
			isBroad:    true,
		},

		// Negative tests - should NOT be detected as broad
		{
			name:       "pythonista is NOT broad (unrelated tool)",
			capability: capabilities.Capability{Kind: "exec", Pattern: "pythonista"},
			isBroad:    false,
		},
		{
			name:       "python-config is NOT broad (utility)",
			capability: capabilities.Capability{Kind: "exec", Pattern: "python-config"},
			isBroad:    false,
		},
		{
			name:       "python_test is NOT broad (underscore)",
			capability: capabilities.Capability{Kind: "exec", Pattern: "python_test"},
			isBroad:    false,
		},
		{
			name:       "ruby-build is NOT broad (utility)",
			capability: capabilities.Capability{Kind: "exec", Pattern: "ruby-build"},
			isBroad:    false,
		},
		{
			name:       "nodejs-dev is NOT broad (package name)",
			capability: capabilities.Capability{Kind: "exec", Pattern: "nodejs-dev"},
			isBroad:    false,
		},
		{
			name:       "/usr/bin/python3.11 is NOT broad (full path)",
			capability: capabilities.Capability{Kind: "exec", Pattern: "/usr/bin/python3.11"},
			isBroad:    false, // Full paths are handled elsewhere
		},
		{
			name:       "specific command is NOT broad",
			capability: capabilities.Capability{Kind: "exec", Pattern: "systemctl status sshd"},
			isBroad:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.capability.IsBroad() // Use domain method
			assert.Equal(t, tt.isBroad, result,
				"Capability %s should %s be detected as broad",
				tt.capability.String(),
				map[bool]string{true: "", false: "NOT"}[tt.isBroad])
		})
	}
}
