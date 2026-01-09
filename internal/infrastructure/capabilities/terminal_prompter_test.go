package capabilities

import (
	"testing"

	"github.com/reglet-dev/reglet/internal/domain/capabilities"
	"github.com/stretchr/testify/assert"
)

func TestTerminalPrompter_IsInteractive(t *testing.T) {
	// Not t.Parallel() because it interacts with os.Stdin
	prompter := NewTerminalPrompter()
	assert.IsType(t, true, prompter.IsInteractive())
}

// TestTerminalPrompter_PromptForCapability is removed as it tests interactive TUI behavior
// which requires specific TUI testing frameworks (like teatest) and cannot be reliably
// tested with simple os.Pipe mocking.
// The logic is now delegated to github.com/charmbracelet/huh.

func TestTerminalPrompter_describeCapability(t *testing.T) {
	t.Parallel()

	prompter := NewTerminalPrompter()

	tests := []struct {
		capability capabilities.Capability
		expected   string
	}{
		{capabilities.Capability{Kind: "network", Pattern: "outbound:*"}, "Network access to any port"},
		{capabilities.Capability{Kind: "network", Pattern: "outbound:private"}, "Network access to private/reserved IPs (localhost, 192.168.x.x, 10.x.x.x, 169.254.169.254, etc.)"},
		{capabilities.Capability{Kind: "network", Pattern: "outbound:80"}, "Network access to port 80"},
		{capabilities.Capability{Kind: "fs", Pattern: "read:/var/log"}, "Read files: /var/log"},
		{capabilities.Capability{Kind: "exec", Pattern: "/bin/sh"}, "Shell execution (executes shell commands)"},
		{capabilities.Capability{Kind: "env", Pattern: "AWS_ACCESS_KEY"}, "Read environment variables: AWS_ACCESS_KEY"},
		{capabilities.Capability{Kind: "unknown", Pattern: "foo"}, "unknown: foo"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, prompter.describeCapability(tt.capability))
		})
	}
}

func TestTerminalPrompter_FormatNonInteractiveError(t *testing.T) {
	t.Parallel()

	prompter := NewTerminalPrompter()
	missing := capabilities.NewGrant()
	missing.Add(capabilities.Capability{Kind: "fs", Pattern: "read:/etc/shadow"})
	missing.Add(capabilities.Capability{Kind: "exec", Pattern: "/usr/bin/sudo"})

	err := prompter.FormatNonInteractiveError(missing)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Plugins require additional permissions")
	assert.Contains(t, err.Error(), "  - Read files: /etc/shadow")
	assert.Contains(t, err.Error(), "  - Execute commands: /usr/bin/sudo")
	assert.Contains(t, err.Error(), "1. Run interactively")
	assert.Contains(t, err.Error(), "2. Use --trust-plugins flag")
	assert.Contains(t, err.Error(), "3. Manually edit: ~/.reglet/config.yaml")
}
