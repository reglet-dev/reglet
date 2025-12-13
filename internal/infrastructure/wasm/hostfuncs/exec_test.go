package hostfuncs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_isShellExecution(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		// Common shells with full paths
		{"sh with path", "/bin/sh", true},
		{"bash with path", "/bin/bash", true},
		{"dash with path", "/bin/dash", true},
		{"zsh with path", "/usr/bin/zsh", true},
		{"ksh with path", "/usr/bin/ksh", true},
		{"csh with path", "/bin/csh", true},
		{"tcsh with path", "/bin/tcsh", true},
		{"fish with path", "/usr/bin/fish", true},

		// Shells without paths
		{"sh bare", "sh", true},
		{"bash bare", "bash", true},
		{"zsh bare", "zsh", true},

		// Non-shell commands
		{"systemctl", "/usr/bin/systemctl", false},
		{"echo", "/bin/echo", false},
		{"ls", "ls", false},
		{"python", "/usr/bin/python3", false},
		{"custom binary", "/opt/myapp/bin/runner", false},

		// Edge cases
		{"shell-like name", "/usr/bin/shiny", false}, // Contains "sh" but not a shell
		{"bashful", "bashful", false},                // Contains "bash" but not a shell
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isShellExecution(tt.command)
			assert.Equal(t, tt.want, got, "isShellExecution(%q) = %v, want %v", tt.command, got, tt.want)
		})
	}
}

// Note: Full integration tests for ExecCommand would require:
// - Creating a WASM module with exec capabilities
// - Setting up CapabilityChecker with test grants
// - Mocking or using real command execution
//
// These are better suited for integration tests rather than unit tests.
// The isShellExecution function is the main logic we can unit test here.
