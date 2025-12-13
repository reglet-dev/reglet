package capabilities

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/whiskeyjimbo/reglet/internal/domain/capabilities"
)

// TerminalPrompter provides interactive terminal prompting for capability grants.
type TerminalPrompter struct{}

// NewTerminalPrompter creates a new TerminalPrompter.
func NewTerminalPrompter() *TerminalPrompter {
	return &TerminalPrompter{}
}

// IsInteractive checks if we're running in an interactive terminal.
func (p *TerminalPrompter) IsInteractive() bool {
	// Check if stdin is a terminal (that's what we're reading from)
	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	// Check if it's a character device (terminal) and not a pipe/file
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// PromptForCapability asks the user whether to grant a capability.
func (p *TerminalPrompter) PromptForCapability(cap capabilities.Capability) (granted bool, always bool, err error) {
	fmt.Fprintf(os.Stderr, "\nPlugin requires permission:\n")
	fmt.Fprintf(os.Stderr, "  ✓ %s\n", p.describeCapability(cap))
	fmt.Fprintf(os.Stderr, "\nAllow this permission? [y/N/always]: ")

	// Create a new buffered reader from stdin
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		// On error (EOF, etc), treat as "no"
		return false, false, nil
	}

	response = strings.ToLower(strings.TrimSpace(response))

	switch response {
	case "y", "yes":
		return true, false, nil
	case "a", "always":
		return true, true, nil
	case "n", "no", "":
		// Empty response (just Enter) counts as "no"
		return false, false, nil
	default:
		// Unknown response - default to deny
		return false, false, nil
	}
}

// describeCapability returns a human-readable description of a capability.
func (p *TerminalPrompter) describeCapability(cap capabilities.Capability) string {
	switch cap.Kind {
	case "network":
		if cap.Pattern == "outbound:*" {
			return "Network access to any port"
		}
		if cap.Pattern == "outbound:private" {
			return "Network access to private/reserved IPs (localhost, 192.168.x.x, 10.x.x.x, 169.254.169.254, etc.)"
		}
		if strings.HasPrefix(cap.Pattern, "outbound:") {
			ports := strings.TrimPrefix(cap.Pattern, "outbound:")
			return fmt.Sprintf("Network access to port %s", ports)
		}
		return fmt.Sprintf("Network: %s", cap.Pattern)
	case "fs":
		if strings.HasPrefix(cap.Pattern, "read:") {
			path := strings.TrimPrefix(cap.Pattern, "read:")
			return fmt.Sprintf("Read files: %s", path)
		}
		if strings.HasPrefix(cap.Pattern, "write:") {
			path := strings.TrimPrefix(cap.Pattern, "write:")
			return fmt.Sprintf("Write files: %s", path)
		}
		return fmt.Sprintf("Filesystem: %s", cap.Pattern)
	case "exec":
		if cap.Pattern == "/bin/sh" {
			return "Shell execution (executes shell commands)"
		}
		return fmt.Sprintf("Execute commands: %s", cap.Pattern)
	case "env":
		return fmt.Sprintf("Read environment variables: %s", cap.Pattern)
	default:
		return fmt.Sprintf("%s: %s", cap.Kind, cap.Pattern)
	}
}

// FormatNonInteractiveError creates a helpful error message for non-interactive mode.
func (p *TerminalPrompter) FormatNonInteractiveError(missing capabilities.Grant) error {
	var msg strings.Builder
	msg.WriteString("Plugins require additional permissions (running in non-interactive mode)\n\n")
	msg.WriteString("Required permissions:\n")

	for _, cap := range missing {
		msg.WriteString(fmt.Sprintf("  - %s\n", p.describeCapability(cap)))
	}

	msg.WriteString("\nTo grant these permissions:\n")
	msg.WriteString("  1. Run interactively and approve when prompted\n")
	msg.WriteString("  2. Use --trust-plugins flag (grants all permissions)\n")
	msg.WriteString(fmt.Sprintf("  3. Manually edit: ~/.reglet/config.yaml\n")) // Hardcode for now, will be dynamic later

	return fmt.Errorf("%s", msg.String())
}
