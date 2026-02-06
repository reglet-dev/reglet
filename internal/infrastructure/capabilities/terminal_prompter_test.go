package capabilities

import (
	"testing"

	"github.com/reglet-dev/reglet/internal/domain/capability"
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

func TestTerminalPrompter_describeGrantSet(t *testing.T) {
	t.Parallel()

	prompter := NewTerminalPrompter()

	tests := []struct {
		name     string
		grantSet capability.GrantSet
		expected []string
	}{
		{
			name: "network capability",
			grantSet: capability.GrantSet{
				Network: &capability.NetworkCapability{
					Rules: []capability.NetworkRule{
						{Hosts: []string{"*"}, Ports: []string{"80"}},
					},
				},
			},
			expected: []string{"Network: hosts=[*], ports=[80]"},
		},
		{
			name: "filesystem read capability",
			grantSet: capability.GrantSet{
				FS: &capability.FileSystemCapability{
					Rules: []capability.FileSystemRule{
						{Read: []string{"/var/log"}},
					},
				},
			},
			expected: []string{"Read files: [/var/log]"},
		},
		{
			name: "exec capability",
			grantSet: capability.GrantSet{
				Exec: &capability.ExecCapability{
					Commands: []string{"/bin/sh"},
				},
			},
			expected: []string{"Execute commands: [/bin/sh]"},
		},
		{
			name: "env capability",
			grantSet: capability.GrantSet{
				Env: &capability.EnvironmentCapability{
					Variables: []string{"AWS_ACCESS_KEY"},
				},
			},
			expected: []string{"Environment variables: [AWS_ACCESS_KEY]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := prompter.describeGrantSet(tt.grantSet)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTerminalPrompter_FormatNonInteractiveErrorForGrantSet(t *testing.T) {
	t.Parallel()

	prompter := NewTerminalPrompter()
	missing := capability.GrantSet{
		FS: &capability.FileSystemCapability{
			Rules: []capability.FileSystemRule{
				{Read: []string{"/etc/shadow"}},
			},
		},
		Exec: &capability.ExecCapability{
			Commands: []string{"/usr/bin/sudo"},
		},
	}

	err := prompter.FormatNonInteractiveError(missing)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Plugins require additional permissions")
	assert.Contains(t, err.Error(), "Read files: [/etc/shadow]")
	assert.Contains(t, err.Error(), "Execute commands: [/usr/bin/sudo]")
	assert.Contains(t, err.Error(), "1. Run interactively")
	assert.Contains(t, err.Error(), "2. Use --trust-plugins flag")
	assert.Contains(t, err.Error(), "3. Manually edit: ~/.reglet/config.yaml")
}
