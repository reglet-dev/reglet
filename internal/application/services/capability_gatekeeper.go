package services

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/domain/capabilities"
	infraCapabilities "github.com/reglet-dev/reglet/internal/infrastructure/capabilities"
)

// CapabilityGatekeeper handles capability granting decisions, user interaction, and persistence.
// This is an application service responsible for the security boundary between required and granted capabilities.
type CapabilityGatekeeper struct {
	fileStore     *infraCapabilities.FileStore
	prompter      *infraCapabilities.TerminalPrompter
	securityLevel string // Security level: strict, standard, permissive
}

// NewCapabilityGatekeeper creates a new capability gatekeeper.
func NewCapabilityGatekeeper(configPath string, securityLevel string) *CapabilityGatekeeper {
	return &CapabilityGatekeeper{
		fileStore:     infraCapabilities.NewFileStore(configPath),
		prompter:      infraCapabilities.NewTerminalPrompter(),
		securityLevel: securityLevel,
	}
}

// GrantCapabilities determines which capabilities to grant based on security policy, user input, and saved grants.
// It handles the complete granting workflow: check saved grants, apply security policy, prompt if needed, persist decisions.
//
// Parameters:
//   - required: capabilities requested by plugins
//   - capabilityInfo: metadata about each capability (is it broad, profile-specific alternative, etc.)
//   - trustAll: if true, auto-grant all capabilities without prompting
//
// Returns:
//   - granted capabilities
//   - error if user denies or security policy blocks
func (g *CapabilityGatekeeper) GrantCapabilities(
	required capabilities.Grant,
	capabilityInfo map[string]ports.CapabilityInfo,
	trustAll bool,
) (capabilities.Grant, error) {
	// If trustAll flag is set, grant everything
	if trustAll {
		slog.Warn("Auto-granting all requested capabilities (--trust-plugins enabled)")
		return required, nil
	}

	// Load existing grants from config file
	existingGrants, err := g.fileStore.Load()
	if err != nil {
		// Config file doesn't exist yet - that's okay
		existingGrants = capabilities.NewGrant()
	}

	// Determine which capabilities are not already granted
	missing := g.findMissingCapabilities(required, existingGrants)

	if len(missing) == 0 {
		// All capabilities already granted
		return existingGrants, nil
	}

	// Non-interactive mode check
	if !g.prompter.IsInteractive() {
		return capabilities.NewGrant(), g.prompter.FormatNonInteractiveError(missing)
	}

	// Interactive prompting for missing capabilities
	newGrants := existingGrants
	shouldSave := false

	for _, capability := range missing {
		// Apply security policy and prompt user
		granted, always, err := g.evaluateCapability(capability, capabilityInfo)
		if err != nil {
			return capabilities.NewGrant(), err
		}

		if granted {
			newGrants.Add(capability)
			if always {
				shouldSave = true
			}
		} else {
			return capabilities.NewGrant(), fmt.Errorf("capability denied by user: %s", capability.String())
		}
	}

	// Save to config if user chose "always" for any capability
	if shouldSave {
		if err := g.fileStore.Save(newGrants); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Warning: failed to save config: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "✓ Permissions saved to %s\n", g.fileStore.ConfigPath())
		}
	}

	return newGrants, nil
}

// evaluateCapability applies security policy and user prompts for a single capability.
// Returns: (granted, saveToConfig, error)
func (g *CapabilityGatekeeper) evaluateCapability(
	capability capabilities.Capability,
	capabilityInfo map[string]ports.CapabilityInfo,
) (bool, bool, error) {
	// Look up metadata for this capability
	key := capability.Kind + ":" + capability.Pattern
	info, hasInfo := capabilityInfo[key]

	// Apply security level policy for broad capabilities
	if hasInfo && info.IsBroad {
		switch g.securityLevel {
		case "strict":
			// Strict mode: auto-deny broad capabilities
			slog.Error("broad capability denied by security policy",
				"level", "strict",
				"capability", capability.String(),
				"risk", capability.RiskDescription())
			return false, false, fmt.Errorf("broad capability denied by strict security policy: %s", capability.String())

		case "permissive":
			// Permissive mode: auto-allow without prompting
			slog.Warn("auto-granting broad capability (permissive mode)",
				"capability", capability.String())
			return true, false, nil

		default: // "standard"
			// Standard mode: warn and prompt
			return g.prompter.PromptForCapabilityWithInfo(
				capability,
				info.IsBroad,
				info.ProfileSpecific,
			)
		}
	}

	// Permissive mode: auto-allow all capabilities
	if g.securityLevel == "permissive" {
		return true, false, nil
	}

	// Standard/strict mode: prompt for non-broad capabilities
	if hasInfo {
		return g.prompter.PromptForCapabilityWithInfo(
			capability,
			info.IsBroad,
			info.ProfileSpecific,
		)
	}

	// Fallback to basic prompt (shouldn't happen in normal flow)
	return g.prompter.PromptForCapability(capability)
}

// findMissingCapabilities returns capabilities in required that are not in granted.
func (g *CapabilityGatekeeper) findMissingCapabilities(required, granted capabilities.Grant) capabilities.Grant {
	missing := capabilities.NewGrant()
	for _, capability := range required {
		if !granted.Contains(capability) {
			missing.Add(capability)
		}
	}
	return missing
}
