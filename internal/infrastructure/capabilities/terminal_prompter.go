package capabilities

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
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
func (p *TerminalPrompter) PromptForCapability(capability capabilities.Capability) (granted bool, always bool, err error) {
	desc := p.describeCapability(capability)

	// Define choices
	const (
		OptionYes    = "Yes, grant for this session"
		OptionAlways = "Always grant (save to config)"
		OptionNo     = "No, deny"
	)

	var selection string

	err = huh.NewSelect[string]().
		Title("Plugin Requesting Permission").
		Description(fmt.Sprintf("✓ %s", desc)).
		Options(
			huh.NewOption(OptionYes, OptionYes),
			huh.NewOption(OptionAlways, OptionAlways),
			huh.NewOption(OptionNo, OptionNo),
		).
		Value(&selection).
		Run()
	if err != nil {
		// Handle cancellation (Ctrl+C) as denial
		return false, false, nil
	}

	switch selection {
	case OptionYes:
		return true, false, nil
	case OptionAlways:
		return true, true, nil
	default:
		return false, false, nil
	}
}

// describeCapability returns a human-readable description of a capability.
func (p *TerminalPrompter) describeCapability(capability capabilities.Capability) string {
	switch capability.Kind {
	case "network":
		if capability.Pattern == "outbound:*" {
			return "Network access to any port"
		}
		if capability.Pattern == "outbound:private" {
			return "Network access to private/reserved IPs (localhost, 192.168.x.x, 10.x.x.x, 169.254.169.254, etc.)"
		}
		if strings.HasPrefix(capability.Pattern, "outbound:") {
			ports := strings.TrimPrefix(capability.Pattern, "outbound:")
			return fmt.Sprintf("Network access to port %s", ports)
		}
		return fmt.Sprintf("Network: %s", capability.Pattern)
	case "fs":
		if strings.HasPrefix(capability.Pattern, "read:") {
			path := strings.TrimPrefix(capability.Pattern, "read:")
			return fmt.Sprintf("Read files: %s", path)
		}
		if strings.HasPrefix(capability.Pattern, "write:") {
			path := strings.TrimPrefix(capability.Pattern, "write:")
			return fmt.Sprintf("Write files: %s", path)
		}
		return fmt.Sprintf("Filesystem: %s", capability.Pattern)
	case "exec":
		if capability.Pattern == "/bin/sh" {
			return "Shell execution (executes shell commands)"
		}
		return fmt.Sprintf("Execute commands: %s", capability.Pattern)
	case "env":
		return fmt.Sprintf("Read environment variables: %s", capability.Pattern)
	default:
		return fmt.Sprintf("%s: %s", capability.Kind, capability.Pattern)
	}
}

// FormatNonInteractiveError creates a helpful error message for non-interactive mode.
func (p *TerminalPrompter) FormatNonInteractiveError(missing capabilities.Grant) error {
	var msg strings.Builder
	msg.WriteString("Plugins require additional permissions (running in non-interactive mode)\n\n")
	msg.WriteString("Required permissions:\n")

	for _, capability := range missing {
		msg.WriteString(fmt.Sprintf("  - %s\n", p.describeCapability(capability)))
	}

	msg.WriteString("\nTo grant these permissions:\n")
	msg.WriteString("  1. Run interactively and approve when prompted\n")
	msg.WriteString("  2. Use --trust-plugins flag (grants all permissions)\n")
	msg.WriteString("  3. Manually edit: ~/.reglet/config.yaml\n") // Hardcode for now, will be dynamic later

	return fmt.Errorf("%s", msg.String())
}
