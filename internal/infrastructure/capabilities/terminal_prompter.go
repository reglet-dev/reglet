package capabilities

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	sdkEntities "github.com/reglet-dev/reglet-sdk/domain/entities"
)

// TerminalPrompter provides interactive terminal prompting for capability grants.
type TerminalPrompter struct {
	riskAnalyzer sdkEntities.RiskAnalyzer
}

// NewTerminalPrompter creates a new TerminalPrompter.
func NewTerminalPrompter() *TerminalPrompter {
	return &TerminalPrompter{
		riskAnalyzer: sdkEntities.NewSimpleRiskAnalyzer(),
	}
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

// PromptForCapability asks the user to grant a capability.
func (p *TerminalPrompter) PromptForCapability(req sdkEntities.CapabilityRequest) (granted bool, always bool, err error) {
	return p.PromptForCapabilityString(req.Description, req.IsBroad)
}

// PromptForCapabilities prompts for multiple capabilities at once.
func (p *TerminalPrompter) PromptForCapabilities(reqs []sdkEntities.CapabilityRequest) (*sdkEntities.GrantSet, error) {
	grants := &sdkEntities.GrantSet{}
	for _, req := range reqs {
		granted, _, err := p.PromptForCapability(req)
		if err != nil {
			return nil, err
		}
		if granted {
			switch req.Kind {
			case "network":
				if rule, ok := req.Rule.(sdkEntities.NetworkRule); ok {
					if grants.Network == nil {
						grants.Network = &sdkEntities.NetworkCapability{}
					}
					grants.Network.Rules = append(grants.Network.Rules, rule)
				}
			case "fs":
				if rule, ok := req.Rule.(sdkEntities.FileSystemRule); ok {
					if grants.FS == nil {
						grants.FS = &sdkEntities.FileSystemCapability{}
					}
					grants.FS.Rules = append(grants.FS.Rules, rule)
				}
			case "env":
				if v, ok := req.Rule.(string); ok {
					if grants.Env == nil {
						grants.Env = &sdkEntities.EnvironmentCapability{}
					}
					grants.Env.Variables = append(grants.Env.Variables, v)
				}
			case "exec":
				if cmd, ok := req.Rule.(string); ok {
					if grants.Exec == nil {
						grants.Exec = &sdkEntities.ExecCapability{}
					}
					grants.Exec.Commands = append(grants.Exec.Commands, cmd)
				}
			}
		}
	}
	return grants, nil
}

// PromptForCapabilityString asks the user whether to grant a capability described by a string.
func (p *TerminalPrompter) PromptForCapabilityString(desc string, isBroad bool) (granted bool, always bool, err error) {
	// Show security warning for broad capabilities
	if isBroad {
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "\033[1;33mSecurity Warning: Broad Permission Requested\033[0m\n\n")
		fmt.Fprintf(os.Stderr, "  %s\n", desc)
		fmt.Fprintf(os.Stderr, "  Recommendation: Review if this broad access is necessary.\n")
		fmt.Fprintf(os.Stderr, "\n")
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
		Description(desc).
		Options(
			huh.NewOption(OptionYes, OptionYes),
			huh.NewOption(OptionAlways, OptionAlways),
			huh.NewOption(OptionNo, OptionNo),
		).
		Value(&selection).
		Run()
	if err != nil {
		// Return error if huh selection failed or was canceled
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

// FormatNonInteractiveError creates a helpful error message for non-interactive mode.
func (p *TerminalPrompter) FormatNonInteractiveError(missing *sdkEntities.GrantSet) error {
	var msg strings.Builder
	msg.WriteString("Plugins require additional permissions (running in non-interactive mode)\n\n")
	msg.WriteString("Required permissions:\n")

	// Describe network capabilities
	if missing.Network != nil {
		for _, rule := range missing.Network.Rules {
			if len(rule.Hosts) > 0 && len(rule.Ports) > 0 {
				msg.WriteString(fmt.Sprintf("  - Network: hosts=%v, ports=%v\n", rule.Hosts, rule.Ports))
			}
		}
	}

	// Describe filesystem capabilities
	if missing.FS != nil {
		for _, rule := range missing.FS.Rules {
			if len(rule.Read) > 0 {
				msg.WriteString(fmt.Sprintf("  - Read files: %v\n", rule.Read))
			}
			if len(rule.Write) > 0 {
				msg.WriteString(fmt.Sprintf("  - Write files: %v\n", rule.Write))
			}
		}
	}

	// Describe environment capabilities
	if missing.Env != nil && len(missing.Env.Variables) > 0 {
		msg.WriteString(fmt.Sprintf("  - Environment variables: %v\n", missing.Env.Variables))
	}

	// Describe exec capabilities
	if missing.Exec != nil && len(missing.Exec.Commands) > 0 {
		msg.WriteString(fmt.Sprintf("  - Execute commands: %v\n", missing.Exec.Commands))
	}

	msg.WriteString("\nTo grant these permissions:\n")
	msg.WriteString("  1. Run interactively and approve when prompted\n")
	msg.WriteString("  2. Use --trust-plugins flag (grants all permissions)\n")
	msg.WriteString("  3. Manually edit: ~/.reglet/config.yaml\n")

	return fmt.Errorf("%s", msg.String())
}

// PromptForProfileTrustWithGrantSet prompts the user to trust a remote profile source.
// Displays the profile URL and required capabilities for informed decision.
func (p *TerminalPrompter) PromptForProfileTrustWithGrantSet(
	url string,
	requiredCaps map[string]*sdkEntities.GrantSet,
) (bool, error) {
	// Build capability description
	var capDescriptions []string
	for plugin, gs := range requiredCaps {
		if gs == nil {
			continue
		}
		descs := p.describeGrantSet(gs)
		for _, desc := range descs {
			capDescriptions = append(capDescriptions, fmt.Sprintf("[%s] %s", plugin, desc))
		}
	}

	// Display warning
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "\033[1;33mRemote Profile Trust Required\033[0m\n\n")
	fmt.Fprintf(os.Stderr, "  Source: %s\n\n", url)

	if len(capDescriptions) > 0 {
		fmt.Fprintf(os.Stderr, "  Required capabilities:\n")
		for _, desc := range capDescriptions {
			fmt.Fprintf(os.Stderr, "    - %s\n", desc)
		}
		fmt.Fprintf(os.Stderr, "\n")
	}

	// Prompt for trust decision
	const (
		OptionYes = "Yes, trust this source for this session"
		OptionNo  = "No, do not run this profile"
	)

	var selection string

	err := huh.NewSelect[string]().
		Title("Trust Remote Profile?").
		Description("This profile is from an untrusted source.").
		Options(
			huh.NewOption(OptionYes, OptionYes),
			huh.NewOption(OptionNo, OptionNo),
		).
		Value(&selection).
		Run()
	if err != nil {
		return false, err
	}

	return selection == OptionYes, nil
}

// describeGrantSet returns human-readable descriptions of a GrantSet.
func (p *TerminalPrompter) describeGrantSet(gs *sdkEntities.GrantSet) []string {
	var descriptions []string

	if gs.Network != nil {
		for _, rule := range gs.Network.Rules {
			descriptions = append(descriptions, fmt.Sprintf("Network: hosts=%v, ports=%v", rule.Hosts, rule.Ports))
		}
	}

	if gs.FS != nil {
		for _, rule := range gs.FS.Rules {
			if len(rule.Read) > 0 {
				descriptions = append(descriptions, fmt.Sprintf("Read files: %v", rule.Read))
			}
			if len(rule.Write) > 0 {
				descriptions = append(descriptions, fmt.Sprintf("Write files: %v", rule.Write))
			}
		}
	}

	if gs.Env != nil && len(gs.Env.Variables) > 0 {
		descriptions = append(descriptions, fmt.Sprintf("Environment variables: %v", gs.Env.Variables))
	}

	if gs.Exec != nil && len(gs.Exec.Commands) > 0 {
		descriptions = append(descriptions, fmt.Sprintf("Execute commands: %v", gs.Exec.Commands))
	}

	return descriptions
}
