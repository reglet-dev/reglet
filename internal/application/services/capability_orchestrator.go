// Package services contains application use cases.
package services

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/whiskeyjimbo/reglet/internal/domain/capabilities"
	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/build"
	infraCapabilities "github.com/whiskeyjimbo/reglet/internal/infrastructure/capabilities"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/wasm"
	"golang.org/x/sync/errgroup"
)

// CapabilityInfo contains metadata about a capability request.
type CapabilityInfo struct {
	Capability      capabilities.Capability
	IsProfileBased  bool   // True if extracted from profile config
	PluginName      string // Which plugin requested this
	IsBroad         bool   // True if pattern is overly permissive
	ProfileSpecific *capabilities.Capability // Profile-specific alternative if available
}

// CapabilityOrchestrator manages capability collection and granting.
// Coordinates domain and infrastructure.
type CapabilityOrchestrator struct {
	fileStore      *infraCapabilities.FileStore
	prompter       *infraCapabilities.TerminalPrompter
	grants         capabilities.Grant
	trustAll       bool                      // Auto-grant all capabilities
	capabilityInfo map[string]CapabilityInfo // Metadata about requested capabilities
	securityLevel  string                    // Security level: strict, standard, permissive
}

// NewCapabilityOrchestrator creates a capability orchestrator with default security level (standard).
func NewCapabilityOrchestrator(trustAll bool) *CapabilityOrchestrator {
	return NewCapabilityOrchestratorWithSecurity(trustAll, "standard")
}

// NewCapabilityOrchestratorWithSecurity creates a capability orchestrator with specified security level.
// securityLevel can be: "strict", "standard", or "permissive"
func NewCapabilityOrchestratorWithSecurity(trustAll bool, securityLevel string) *CapabilityOrchestrator {
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".reglet", "config.yaml")

	prompter := infraCapabilities.NewTerminalPrompter()
	return &CapabilityOrchestrator{
		fileStore:      infraCapabilities.NewFileStore(configPath),
		prompter:       prompter,
		grants:         capabilities.NewGrant(),
		trustAll:       trustAll,
		capabilityInfo: make(map[string]CapabilityInfo),
		securityLevel:  securityLevel,
	}
}

// CollectCapabilities creates a temporary runtime and collects required capabilities.
// Returns the required capabilities and the temporary runtime (caller must close it).
func (o *CapabilityOrchestrator) CollectCapabilities(ctx context.Context, profile *entities.Profile, pluginDir string) (map[string][]capabilities.Capability, *wasm.Runtime, error) {
	// Create temporary runtime for capability collection
	runtime, err := wasm.NewRuntime(ctx, build.Get())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temporary runtime: %w", err)
	}

	caps, err := o.CollectRequiredCapabilities(ctx, profile, runtime, pluginDir)
	if err != nil {
		runtime.Close(ctx)
		return nil, nil, err
	}

	return caps, runtime, nil
}

// CollectRequiredCapabilities loads plugins and identifies requirements.
// It prioritizes specific capabilities extracted from profile configs over plugin metadata.
func (o *CapabilityOrchestrator) CollectRequiredCapabilities(ctx context.Context, profile *entities.Profile, runtime *wasm.Runtime, pluginDir string) (map[string][]capabilities.Capability, error) {
	// First, extract specific capabilities from profile observation configs
	profileCaps := o.extractProfileCapabilities(profile)

	// Get unique plugin names from profile
	pluginNames := make(map[string]bool)
	for _, ctrl := range profile.Controls.Items {
		for _, obs := range ctrl.Observations {
			pluginNames[obs.Plugin] = true
		}
	}

	// Convert to slice for parallel iteration
	names := make([]string, 0, len(pluginNames))
	for name := range pluginNames {
		names = append(names, name)
	}

	// Thread-safe map for collecting plugin metadata capabilities
	var mu sync.Mutex
	pluginMetaCaps := make(map[string][]capabilities.Capability)

	// Load plugins in parallel to get their declared capabilities
	g, gctx := errgroup.WithContext(ctx)
	for _, name := range names {
		name := name // Capture loop variable
		g.Go(func() error {
			// Plugin name is validated in config.validatePluginName() to prevent path traversal
			pluginPath := filepath.Join(pluginDir, name, name+".wasm")
			wasmBytes, err := os.ReadFile(pluginPath)
			if err != nil {
				return fmt.Errorf("failed to read plugin %s: %w", name, err)
			}

			// Load plugin (we need a temporary runtime with no capabilities for this)
			plugin, err := runtime.LoadPlugin(gctx, name, wasmBytes)
			if err != nil {
				return fmt.Errorf("failed to load plugin %s: %w", name, err)
			}

			// Get plugin metadata
			info, err := plugin.Describe(gctx)
			if err != nil {
				return fmt.Errorf("failed to get capabilities from plugin %s: %w", name, err)
			}

			// Collect plugin declared capabilities (thread-safe)
			mu.Lock()
			var caps []capabilities.Capability
			for _, capability := range info.Capabilities {
				caps = append(caps, capabilities.Capability{
					Kind:    capability.Kind,
					Pattern: capability.Pattern,
				})
			}
			pluginMetaCaps[name] = caps
			mu.Unlock()

			return nil
		})
	}

	// Wait for all plugins to load
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Merge profile-extracted capabilities with plugin metadata
	// Profile-extracted capabilities take precedence (more specific)
	required := make(map[string][]capabilities.Capability)

	// Clear and rebuild capability info metadata
	o.capabilityInfo = make(map[string]CapabilityInfo)

	for name := range pluginNames {
		profileSpecific, hasProfile := profileCaps[name]
		metaCaps, hasMeta := pluginMetaCaps[name]

		// Start with profile-extracted capabilities
		if hasProfile && len(profileSpecific) > 0 {
			// Use specific capabilities from profile analysis
			required[name] = profileSpecific
			slog.Debug("using profile-extracted capabilities",
				"plugin", name,
				"count", len(profileSpecific),
				"capabilities", profileSpecific)

			// Store metadata for each profile-specific capability
			for _, cap := range profileSpecific {
				key := cap.Kind + ":" + cap.Pattern
				o.capabilityInfo[key] = CapabilityInfo{
					Capability:      cap,
					IsProfileBased:  true,
					PluginName:      name,
					IsBroad:         isBroadCapability(cap),
					ProfileSpecific: nil, // Already profile-specific
				}
			}

		} else if hasMeta {
			// Fallback to plugin metadata if we couldn't extract specific requirements
			required[name] = metaCaps
			slog.Debug("using plugin metadata capabilities (fallback)",
				"plugin", name,
				"count", len(metaCaps),
				"capabilities", metaCaps)

			// Store metadata for plugin-declared capabilities
			for _, cap := range metaCaps {
				key := cap.Kind + ":" + cap.Pattern
				info := CapabilityInfo{
					Capability:     cap,
					IsProfileBased: false,
					PluginName:     name,
					IsBroad:        isBroadCapability(cap),
				}

				// Check if there's a profile-specific alternative we could have used
				if hasProfile && len(profileSpecific) > 0 {
					// Use first profile-specific cap as the alternative
					alt := profileSpecific[0]
					info.ProfileSpecific = &alt
				}

				o.capabilityInfo[key] = info
			}
		}
	}

	return required, nil
}

// extractProfileCapabilities analyzes profile observations to extract specific capability requirements.
// This enables principle of least privilege by requesting only the resources actually used,
// rather than the plugin's full declared capabilities.
func (o *CapabilityOrchestrator) extractProfileCapabilities(profile *entities.Profile) map[string][]capabilities.Capability {
	// Use map to deduplicate capabilities per plugin
	profileCaps := make(map[string]map[string]capabilities.Capability)

	// Analyze each control's observations
	for _, ctrl := range profile.Controls.Items {
		for _, obs := range ctrl.Observations {
			pluginName := obs.Plugin

			// Initialize plugin entry if needed
			if _, ok := profileCaps[pluginName]; !ok {
				profileCaps[pluginName] = make(map[string]capabilities.Capability)
			}

			// Extract plugin-specific capabilities based on config
			var extractedCaps []capabilities.Capability

			switch pluginName {
			case "file":
				// Extract file path from config
				if pathVal, ok := obs.Config["path"]; ok {
					if path, ok := pathVal.(string); ok && path != "" {
						// Create specific read capability for this file
						extractedCaps = append(extractedCaps, capabilities.Capability{
							Kind:    "fs",
							Pattern: "read:" + path,
						})
					}
				}

			case "command":
				// Extract command from config
				if cmdVal, ok := obs.Config["command"]; ok {
					if cmd, ok := cmdVal.(string); ok && cmd != "" {
						extractedCaps = append(extractedCaps, capabilities.Capability{
							Kind:    "exec",
							Pattern: cmd,
						})
					}
				}

			case "http", "tcp", "dns":
				// Network plugins - extract specific endpoints if available
				if urlVal, ok := obs.Config["url"]; ok {
					if url, ok := urlVal.(string); ok && url != "" {
						extractedCaps = append(extractedCaps, capabilities.Capability{
							Kind:    "network",
							Pattern: "outbound:" + url,
						})
					}
				} else if hostVal, ok := obs.Config["host"]; ok {
					if host, ok := hostVal.(string); ok && host != "" {
						extractedCaps = append(extractedCaps, capabilities.Capability{
							Kind:    "network",
							Pattern: "outbound:" + host,
						})
					}
				}
			}

			// Deduplicate by using capability string as key
			for _, cap := range extractedCaps {
				key := cap.Kind + ":" + cap.Pattern
				profileCaps[pluginName][key] = cap
			}
		}
	}

	// Convert map to slice
	result := make(map[string][]capabilities.Capability)
	for pluginName, capMap := range profileCaps {
		caps := make([]capabilities.Capability, 0, len(capMap))
		for _, cap := range capMap {
			caps = append(caps, cap)
		}
		if len(caps) > 0 {
			result[pluginName] = caps
		}
	}

	return result
}

// isBroadCapability checks if a capability pattern is overly permissive.
func isBroadCapability(cap capabilities.Capability) bool {
	// Filesystem broad patterns
	if cap.Kind == "fs" {
		broadPatterns := []string{
			"**",           // All files (legacy)
			"/**",          // Root filesystem
			"read:**",      // All files read
			"write:**",     // All files write
			"read:/",       // Root read
			"write:/",      // Root write
			"read:/etc/**", // Sensitive system config
			"write:/etc/**",
			"read:/root/**", // Root home directory
			"write:/root/**",
			"read:/home/**", // All user home directories
			"write:/home/**",
		}
		for _, pattern := range broadPatterns {
			if cap.Pattern == pattern {
				return true
			}
		}
	}

	// Execution broad patterns
	if cap.Kind == "exec" {
		// Any shell interpreter is broad
		shells := []string{"bash", "sh", "zsh", "fish", "/bin/bash", "/bin/sh"}
		for _, shell := range shells {
			if cap.Pattern == shell {
				return true
			}
		}
	}

	// Network broad patterns
	if cap.Kind == "network" {
		// Wildcard hosts or ports
		if cap.Pattern == "*" || cap.Pattern == "outbound:*" {
			return true
		}
	}

	return false
}

// GrantCapabilities resolves permissions via file or prompt.
func (o *CapabilityOrchestrator) GrantCapabilities(required map[string][]capabilities.Capability, trustAll bool) (map[string][]capabilities.Capability, error) {
	// Flatten all required capabilities to a unique set for user prompting
	flatRequired := capabilities.NewGrant()
	for _, caps := range required {
		for _, capability := range caps {
			flatRequired.Add(capability)
		}
	}

	// If trustAll flag is set (from constructor or parameter), grant everything
	if o.trustAll || trustAll {
		slog.Warn("Auto-granting all requested capabilities (--trust-plugins enabled)")
		o.grants = flatRequired
		return required, nil
	}

	// Load existing grants from config file
	existingGrants, err := o.fileStore.Load()
	if err != nil {
		// Config file doesn't exist yet - that's okay
		existingGrants = capabilities.NewGrant()
	}

	// Determine which capabilities are not already granted
	missing := o.findMissingCapabilities(flatRequired, existingGrants)

	var grantedGlobal capabilities.Grant
	if len(missing) == 0 {
		// All capabilities already granted
		grantedGlobal = existingGrants
	} else {
		// Prompt for missing capabilities
		if !o.prompter.IsInteractive() {
			// Non-interactive mode - fail with clear instructions
			return nil, o.prompter.FormatNonInteractiveError(missing)
		}

		// Interactive prompts
		newGrants := existingGrants
		shouldSave := false

		for _, capability := range missing {
			// Look up metadata for this capability
			key := capability.Kind + ":" + capability.Pattern
			info, hasInfo := o.capabilityInfo[key]

			var granted bool
			var always bool
			var err error

			// Apply security level policy
			if hasInfo && info.IsBroad {
				switch o.securityLevel {
				case "strict":
					// Strict mode: auto-deny broad capabilities
					slog.Error("broad capability denied by security policy",
						"level", "strict",
						"capability", capability.String(),
						"risk", o.describeBroadRisk(capability))
					return nil, fmt.Errorf("broad capability denied by strict security policy: %s", capability.String())

				case "permissive":
					// Permissive mode: auto-allow without prompting
					slog.Warn("auto-granting broad capability (permissive mode)",
						"capability", capability.String())
					granted = true
					always = false

				default: // "standard"
					// Standard mode: warn and prompt
					granted, always, err = o.prompter.PromptForCapabilityWithInfo(
						capability,
						info.IsBroad,
						info.ProfileSpecific,
					)
				}
			} else if o.securityLevel == "permissive" {
				// Permissive mode: auto-allow all capabilities without prompting
				granted = true
				always = false
			} else {
				// Standard/strict mode: prompt for non-broad capabilities
				if hasInfo {
					granted, always, err = o.prompter.PromptForCapabilityWithInfo(
						capability,
						info.IsBroad,
						info.ProfileSpecific,
					)
				} else {
					// Fallback to basic prompt (shouldn't happen in normal flow)
					granted, always, err = o.prompter.PromptForCapability(capability)
				}
			}

			if err != nil {
				return nil, err
			}

			if granted {
				newGrants.Add(capability)
				if always {
					shouldSave = true
				}
			} else {
				return nil, fmt.Errorf("capability denied by user: %s", capability.String())
			}
		}

		// Save to config if user chose "always" for any capability
		if shouldSave {
			if err := o.fileStore.Save(newGrants); err != nil {
				fmt.Fprintf(os.Stderr, "⚠️  Warning: failed to save config: %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "✓ Permissions saved to %s\n", o.fileStore.ConfigPath())
			}
		}
		grantedGlobal = newGrants
	}

	o.grants = grantedGlobal

	// Filter the requested capabilities against the globally granted ones
	// ensuring each plugin only gets what it requested AND what was granted
	grantedPerPlugin := make(map[string][]capabilities.Capability)
	for name, caps := range required {
		var allowed capabilities.Grant
		for _, capability := range caps {
			if grantedGlobal.Contains(capability) {
				allowed.Add(capability)
			}
		}
		if len(allowed) > 0 {
			grantedPerPlugin[name] = allowed
		}
	}

	return grantedPerPlugin, nil
}

// findMissingCapabilities returns capabilities in required that are not in granted.
func (o *CapabilityOrchestrator) findMissingCapabilities(required, granted capabilities.Grant) capabilities.Grant {
	missing := capabilities.NewGrant()
	for _, capability := range required {
		if !granted.Contains(capability) {
			missing.Add(capability)
		}
	}
	return missing
}

// describeBroadRisk explains the security implications of a broad capability.
func (o *CapabilityOrchestrator) describeBroadRisk(cap capabilities.Capability) string {
	switch cap.Kind {
	case "fs":
		if strings.Contains(cap.Pattern, "/**") || strings.Contains(cap.Pattern, "**") {
			return "Plugin can access ALL files on the system"
		}
		if strings.Contains(cap.Pattern, "/etc") {
			return "Plugin can access sensitive system configuration"
		}
		if strings.Contains(cap.Pattern, "/root") || strings.Contains(cap.Pattern, "/home") {
			return "Plugin can access user home directories and private files"
		}
	case "exec":
		if cap.Pattern == "bash" || cap.Pattern == "sh" || strings.Contains(cap.Pattern, "/bin/") {
			return "Plugin can execute arbitrary shell commands"
		}
	case "network":
		if cap.Pattern == "*" || cap.Pattern == "outbound:*" {
			return "Plugin can connect to any host on the internet"
		}
	}
	return "Plugin has broad access beyond what may be necessary"
}
