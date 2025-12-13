package capabilities

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/whiskeyjimbo/reglet/internal/domain/capabilities"
)

func TestTerminalPrompter_IsInteractive(t *testing.T) {
	// Not t.Parallel() because it interacts with os.Stdin
	prompter := NewTerminalPrompter()
	assert.IsType(t, true, prompter.IsInteractive())
}

func TestTerminalPrompter_PromptForCapability(t *testing.T) {
	t.Parallel()

	prompter := NewTerminalPrompter()
	cap := capabilities.Capability{Kind: "fs", Pattern: "read:/etc/passwd"}

	tests := []struct {
		name         string
		input        string
		expectedGranted bool
		expectedAlways  bool
	}{
		{"yes", "y\n", true, false},
		{"Yes", "Yes\n", true, false},
		{"Y", "Y\n", true, false},
		{"always", "always\n", true, true},
		{"Always", "Always\n", true, true},
		{"A", "A\n", true, true},
		{"no", "n\n", false, false},
		{"No", "No\n", false, false},
		{"N", "N\n", false, false},
		{"empty", "\n", false, false},
		{"other", "foo\n", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock os.Stdin
			oldStdin := os.Stdin
			defer func() { os.Stdin = oldStdin }()

			r, w, err := os.Pipe()
			assert.NoError(t, err)
			os.Stdin = r
			
			            _, err = w.WriteString(tt.input)
			            assert.NoError(t, err)
			            w.Close()
			
			            // Capture stderr output
			            oldStderr := os.Stderr
			            defer func() { os.Stderr = oldStderr }()
			            
			            r2, w2, err := os.Pipe()
			            assert.NoError(t, err)
			            os.Stderr = w2
			            
			            granted, always, err := prompter.PromptForCapability(cap)
			            w2.Close() // Close writer after prompt
			
			            output, _ := io.ReadAll(r2) // Read all content from the pipe
			            r2.Close() // Close reader
			
			            assert.NoError(t, err)
			            assert.Equal(t, tt.expectedGranted, granted)
			            assert.Equal(t, tt.expectedAlways, always)
			
			            // Read captured stderr for verification
			                        assert.Contains(t, string(output), "Plugin requires permission")
			                        assert.Contains(t, string(output), "Read files: /etc/passwd")
			            		})
			            	}
			            }
			            
			            func TestTerminalPrompter_describeCapability(t *testing.T) {
			            	t.Parallel()
			            
			            	prompter := NewTerminalPrompter()
			            
			            	tests := []struct {
			            		cap      capabilities.Capability
			            		expected string
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
			            			assert.Equal(t, tt.expected, prompter.describeCapability(tt.cap))
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
