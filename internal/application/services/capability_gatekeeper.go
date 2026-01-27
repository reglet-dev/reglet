package services

import (
	"fmt"
	"log/slog"
	"os"

	sdkEntities "github.com/reglet-dev/reglet-sdk/go/domain/entities"
	grantstore "github.com/reglet-dev/reglet-sdk/go/infrastructure/grantstore"
	"github.com/reglet-dev/reglet/internal/application/ports"
	infraCapabilities "github.com/reglet-dev/reglet/internal/infrastructure/capabilities"
)

// CapabilityGatekeeper handles capability granting decisions, user interaction, and persistence.
// This is an application service responsible for the security boundary between required and granted capabilities.
type CapabilityGatekeeper struct {
	grantStore    ports.GrantStore
	prompter      ports.Prompter // Use interface
	riskAssessor  *sdkEntities.RiskAssessor
	securityLevel string // Security level: strict, standard, permissive
}

// NewCapabilityGatekeeper creates a new capability gatekeeper.
func NewCapabilityGatekeeper(configPath string, securityLevel string) *CapabilityGatekeeper {
	return &CapabilityGatekeeper{
		grantStore:    grantstore.NewFileStore(grantstore.WithPath(configPath)),
		prompter:      infraCapabilities.NewTerminalPrompter(),
		riskAssessor:  sdkEntities.NewRiskAssessor(),
		securityLevel: securityLevel,
	}
}

// GrantCapabilities determines which capabilities to grant based on security policy, user input, and saved grants.
// It handles the complete granting workflow: check saved grants, apply security policy, prompt if needed, persist decisions.
//
// Parameters:
//   - required: capabilities requested by plugins (as GrantSet)
//   - capabilityInfo: metadata about each capability (is it broad, profile-specific alternative, etc.)
//   - trustAll: if true, auto-grant all capabilities without prompting
//
// Returns:
//   - granted capabilities (as GrantSet)
//   - error if user denies or security policy blocks
func (g *CapabilityGatekeeper) GrantCapabilities(
	required *sdkEntities.GrantSet,
	capabilityInfo map[string]ports.CapabilityInfo,
	trustAll bool,
) (*sdkEntities.GrantSet, error) {
	// If trustAll flag is set, grant everything
	if trustAll {
		slog.Warn("Auto-granting all requested capabilities (--trust-plugins enabled)")
		return required, nil
	}

	// Load existing grants from config file
	existingGrants, err := g.grantStore.Load()
	if err != nil {
		// Config file doesn't exist yet - that's okay
		existingGrants = &sdkEntities.GrantSet{}
	}

	// Determine which capabilities are not already granted
	missing := required.Difference(existingGrants)

	if missing == nil || missing.IsEmpty() {
		// All capabilities already granted
		return existingGrants, nil
	}

	// Non-interactive mode check
	if !g.prompter.IsInteractive() {
		return &sdkEntities.GrantSet{}, g.prompter.FormatNonInteractiveError(missing)
	}

	// Interactive prompting for missing capabilities
	newGrants := existingGrants.Clone()
	shouldSave := false

	// Prompt for each type of capability
	if err := g.promptForCapabilities(missing, capabilityInfo, newGrants, &shouldSave); err != nil {
		return &sdkEntities.GrantSet{}, err
	}

	// Save to config if user chose "always" for any capability
	if shouldSave {
		if err := g.grantStore.Save(newGrants); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Warning: failed to save config: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "✓ Permissions saved to %s\n", g.grantStore.ConfigPath())
		}
	}

	return newGrants, nil
}

// promptForCapabilities prompts the user for each type of missing capability.
func (g *CapabilityGatekeeper) promptForCapabilities(
	missing *sdkEntities.GrantSet,
	capabilityInfo map[string]ports.CapabilityInfo,
	newGrants *sdkEntities.GrantSet,
	shouldSave *bool,
) error {
	if err := g.promptForNetwork(missing, capabilityInfo, newGrants, shouldSave); err != nil {
		return err
	}
	if err := g.promptForFS(missing, capabilityInfo, newGrants, shouldSave); err != nil {
		return err
	}
	if err := g.promptForEnv(missing, capabilityInfo, newGrants, shouldSave); err != nil {
		return err
	}
	return g.promptForExec(missing, capabilityInfo, newGrants, shouldSave)
}

func (g *CapabilityGatekeeper) promptForNetwork(
	missing *sdkEntities.GrantSet,
	capabilityInfo map[string]ports.CapabilityInfo,
	newGrants *sdkEntities.GrantSet,
	shouldSave *bool,
) error {
	if missing.Network == nil {
		return nil
	}
	for _, rule := range missing.Network.Rules {
		granted, always, err := g.evaluateNetworkRule(rule, capabilityInfo)
		if err != nil {
			return err
		}
		if granted {
			if newGrants.Network == nil {
				newGrants.Network = &sdkEntities.NetworkCapability{}
			}
			newGrants.Network.Rules = append(newGrants.Network.Rules, rule)
			if always {
				*shouldSave = true
			}
		} else {
			return fmt.Errorf("capability denied by user: network %v:%v", rule.Hosts, rule.Ports)
		}
	}
	return nil
}

func (g *CapabilityGatekeeper) promptForFS(
	missing *sdkEntities.GrantSet,
	capabilityInfo map[string]ports.CapabilityInfo,
	newGrants *sdkEntities.GrantSet,
	shouldSave *bool,
) error {
	if missing.FS == nil {
		return nil
	}
	for _, rule := range missing.FS.Rules {
		for _, path := range rule.Read {
			granted, always, err := g.evaluateFSPath("read", path, capabilityInfo)
			if err != nil {
				return err
			}
			if granted {
				if newGrants.FS == nil {
					newGrants.FS = &sdkEntities.FileSystemCapability{}
				}
				newGrants.FS.Rules = append(newGrants.FS.Rules, sdkEntities.FileSystemRule{Read: []string{path}})
				if always {
					*shouldSave = true
				}
			} else {
				return fmt.Errorf("capability denied by user: fs read:%s", path)
			}
		}
		for _, path := range rule.Write {
			granted, always, err := g.evaluateFSPath("write", path, capabilityInfo)
			if err != nil {
				return err
			}
			if granted {
				if newGrants.FS == nil {
					newGrants.FS = &sdkEntities.FileSystemCapability{}
				}
				newGrants.FS.Rules = append(newGrants.FS.Rules, sdkEntities.FileSystemRule{Write: []string{path}})
				if always {
					*shouldSave = true
				}
			} else {
				return fmt.Errorf("capability denied by user: fs write:%s", path)
			}
		}
	}
	return nil
}

func (g *CapabilityGatekeeper) promptForEnv(
	missing *sdkEntities.GrantSet,
	capabilityInfo map[string]ports.CapabilityInfo,
	newGrants *sdkEntities.GrantSet,
	shouldSave *bool,
) error {
	if missing.Env == nil {
		return nil
	}
	for _, v := range missing.Env.Variables {
		granted, always, err := g.evaluateEnvVar(v, capabilityInfo)
		if err != nil {
			return err
		}
		if granted {
			if newGrants.Env == nil {
				newGrants.Env = &sdkEntities.EnvironmentCapability{}
			}
			newGrants.Env.Variables = append(newGrants.Env.Variables, v)
			if always {
				*shouldSave = true
			}
		} else {
			return fmt.Errorf("capability denied by user: env %s", v)
		}
	}
	return nil
}

func (g *CapabilityGatekeeper) promptForExec(
	missing *sdkEntities.GrantSet,
	capabilityInfo map[string]ports.CapabilityInfo,
	newGrants *sdkEntities.GrantSet,
	shouldSave *bool,
) error {
	if missing.Exec == nil {
		return nil
	}
	for _, cmd := range missing.Exec.Commands {
		granted, always, err := g.evaluateExecCmd(cmd, capabilityInfo)
		if err != nil {
			return err
		}
		if granted {
			if newGrants.Exec == nil {
				newGrants.Exec = &sdkEntities.ExecCapability{}
			}
			newGrants.Exec.Commands = append(newGrants.Exec.Commands, cmd)
			if always {
				*shouldSave = true
			}
		} else {
			return fmt.Errorf("capability denied by user: exec %s", cmd)
		}
	}
	return nil
}

// evaluateNetworkRule evaluates a network rule and prompts if needed.
func (g *CapabilityGatekeeper) evaluateNetworkRule(
	rule sdkEntities.NetworkRule,
	capabilityInfo map[string]ports.CapabilityInfo,
) (bool, bool, error) {
	// Check if this is a broad capability
	isBroad := len(rule.Hosts) == 1 && rule.Hosts[0] == "*" && len(rule.Ports) == 1 && rule.Ports[0] == "*"

	// Get risk description
	gs := &sdkEntities.GrantSet{Network: &sdkEntities.NetworkCapability{Rules: []sdkEntities.NetworkRule{rule}}}

	req := sdkEntities.CapabilityRequest{
		Kind:        "network",
		Rule:        rule,
		Description: fmt.Sprintf("network %v:%v", rule.Hosts, rule.Ports),
		IsBroad:     isBroad,
	}

	return g.evaluateWithSecurityLevel(req, g.riskAssessor.DescribeRisks(gs))
}

// evaluateFSPath evaluates a filesystem path and prompts if needed.
func (g *CapabilityGatekeeper) evaluateFSPath(
	op, path string,
	capabilityInfo map[string]ports.CapabilityInfo,
) (bool, bool, error) {
	isBroad := path == "/**" || path == "**"
	gs := &sdkEntities.GrantSet{}
	var rule sdkEntities.FileSystemRule
	if op == "read" {
		rule = sdkEntities.FileSystemRule{Read: []string{path}}
		gs.FS = &sdkEntities.FileSystemCapability{Rules: []sdkEntities.FileSystemRule{rule}}
	} else {
		rule = sdkEntities.FileSystemRule{Write: []string{path}}
		gs.FS = &sdkEntities.FileSystemCapability{Rules: []sdkEntities.FileSystemRule{rule}}
	}

	req := sdkEntities.CapabilityRequest{
		Kind:        "fs",
		Rule:        rule,
		Description: fmt.Sprintf("fs %s:%s", op, path),
		IsBroad:     isBroad,
	}

	return g.evaluateWithSecurityLevel(req, g.riskAssessor.DescribeRisks(gs))
}

// evaluateEnvVar evaluates an environment variable and prompts if needed.
func (g *CapabilityGatekeeper) evaluateEnvVar(
	v string,
	capabilityInfo map[string]ports.CapabilityInfo,
) (bool, bool, error) {
	isBroad := v == "*"
	gs := &sdkEntities.GrantSet{Env: &sdkEntities.EnvironmentCapability{Variables: []string{v}}}

	req := sdkEntities.CapabilityRequest{
		Kind:        "env",
		Rule:        v,
		Description: fmt.Sprintf("env %s", v),
		IsBroad:     isBroad,
	}

	return g.evaluateWithSecurityLevel(req, g.riskAssessor.DescribeRisks(gs))
}

// evaluateExecCmd evaluates an exec command and prompts if needed.
func (g *CapabilityGatekeeper) evaluateExecCmd(
	cmd string,
	capabilityInfo map[string]ports.CapabilityInfo,
) (bool, bool, error) {
	isBroad := cmd == "**" || cmd == "*"
	gs := &sdkEntities.GrantSet{Exec: &sdkEntities.ExecCapability{Commands: []string{cmd}}}

	req := sdkEntities.CapabilityRequest{
		Kind:        "exec",
		Rule:        cmd,
		Description: fmt.Sprintf("exec %s", cmd),
		IsBroad:     isBroad,
	}

	return g.evaluateWithSecurityLevel(req, g.riskAssessor.DescribeRisks(gs))
}

// evaluateWithSecurityLevel applies security level policy and prompts if needed.
func (g *CapabilityGatekeeper) evaluateWithSecurityLevel(req sdkEntities.CapabilityRequest, risks []string) (bool, bool, error) {
	riskDesc := ""
	if len(risks) > 0 {
		riskDesc = risks[0]
	}

	if req.IsBroad {
		switch g.securityLevel {
		case "strict":
			// Strict mode: auto-deny broad capabilities
			if riskDesc == "" {
				riskDesc = "broad access beyond what may be necessary"
			}
			slog.Error("broad capability denied by security policy",
				"level", "strict",
				"capability", req.Description,
				"risk", riskDesc)
			return false, false, fmt.Errorf("broad capability denied by strict security policy: %s", req.Description)

		case "permissive":
			// Permissive mode: auto-allow without prompting
			slog.Warn("auto-granting broad capability (permissive mode)",
				"capability", req.Description)
			return true, false, nil
		}
	}

	// Permissive mode: auto-allow all capabilities
	if g.securityLevel == "permissive" {
		return true, false, nil
	}

	// Standard/strict mode: prompt for capabilities
	return g.prompter.PromptForCapability(req)
}
