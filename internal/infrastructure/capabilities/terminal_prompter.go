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
	return p.PromptForCapabilityWithInfo(capability, false, nil)
}

// PromptForCapabilityWithInfo asks the user whether to grant a capability with security warnings.
func (p *TerminalPrompter) PromptForCapabilityWithInfo(
	capability capabilities.Capability,
	isBroad bool,
	profileSpecific *capabilities.Capability,
) (granted bool, always bool, err error) {
	desc := p.describeCapability(capability)

	// Show security warning for broad capabilities
	if isBroad {
		p.displayBroadCapabilityWarning(capability, profileSpecific)
	}

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
		// Return error if huh selection failed or was cancelled
		return false, false, err
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

// displayBroadCapabilityWarning shows a security warning for overly broad capabilities.
func (p *TerminalPrompter) displayBroadCapabilityWarning(
	broad capabilities.Capability,
	profileSpecific *capabilities.Capability,
) {
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "⚠️  \033[1;33mSecurity Warning: Broad Permission Requested\033[0m\n\n")

	// Show what's being requested
	fmt.Fprintf(os.Stderr, "  Requested: %s\n", p.describeCapability(broad))

	// Explain the risk
	risk := p.describeBroadRisk(broad)
	if risk != "" {
		fmt.Fprintf(os.Stderr, "  Risk: %s\n", risk)
	}

	// Show profile-specific alternative if available
	if profileSpecific != nil {
		fmt.Fprintf(os.Stderr, "\n  ✓ Profile only needs: %s\n", p.describeCapability(*profileSpecific))
		fmt.Fprintf(os.Stderr, "  Recommendation: Consider granting only what the profile needs.\n")
	} else {
		fmt.Fprintf(os.Stderr, "  Recommendation: Review if this broad access is necessary.\n")
	}

	fmt.Fprintf(os.Stderr, "\n")
}

// describeBroadRisk explains the security implications of a broad capability.
func (p *TerminalPrompter) describeBroadRisk(capability capabilities.Capability) string {
	switch capability.Kind {
	case "fs":
		if strings.Contains(capability.Pattern, "/**") || strings.Contains(capability.Pattern, "**") {
			return "Plugin can access ALL files on the system"
		}
		if strings.Contains(capability.Pattern, "/etc") {
			return "Plugin can access sensitive system configuration"
		}
		if strings.Contains(capability.Pattern, "/root") || strings.Contains(capability.Pattern, "/home") {
			return "Plugin can access user home directories and private files"
		}
	case "exec":
		if capability.Pattern == "bash" || capability.Pattern == "sh" || strings.Contains(capability.Pattern, "/bin/") {
			return "Plugin can execute arbitrary shell commands"
		}
	case "network":
		if capability.Pattern == "*" || capability.Pattern == "outbound:*" {
			return "Plugin can connect to any host on the internet"
		}
	}
	return "Plugin has broad access beyond what may be necessary"
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
