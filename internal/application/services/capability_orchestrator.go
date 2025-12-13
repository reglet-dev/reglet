// Package services contains application use cases.
package services

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/whiskeyjimbo/reglet/internal/domain/capabilities"
	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
	infraCapabilities "github.com/whiskeyjimbo/reglet/internal/infrastructure/capabilities"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/wasm"
	"golang.org/x/sync/errgroup"
)

// CapabilityOrchestrator orchestrates capability collection and granting workflow.
// This is application-layer orchestration combining domain and infrastructure concerns.
type CapabilityOrchestrator struct {
	fileStore *infraCapabilities.FileStore
	prompter  *infraCapabilities.TerminalPrompter
	grants    capabilities.Grant
	trustAll  bool // Auto-grant all capabilities if true
}

// NewCapabilityOrchestrator creates a capability orchestrator.
func NewCapabilityOrchestrator(trustAll bool) *CapabilityOrchestrator {
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".reglet", "config.yaml")

	prompter := infraCapabilities.NewTerminalPrompter()
	return &CapabilityOrchestrator{
		fileStore: infraCapabilities.NewFileStore(configPath),
		prompter:  prompter,
		grants:    capabilities.NewGrant(),
		trustAll:  trustAll,
	}
}

// CollectRequiredCapabilities loads all plugins in parallel and collects their required capabilities.
func (o *CapabilityOrchestrator) CollectRequiredCapabilities(ctx context.Context, profile *entities.Profile, runtime *wasm.Runtime, pluginDir string) (map[string][]capabilities.Capability, error) {
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

	// Thread-safe map for collecting capabilities
	var mu sync.Mutex
	required := make(map[string][]capabilities.Capability)

	// Load plugins in parallel
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

			// Collect capabilities (thread-safe)
			mu.Lock()
			var caps []capabilities.Capability
			for _, cap := range info.Capabilities {
				caps = append(caps, capabilities.Capability{
					Kind:    cap.Kind,
					Pattern: cap.Pattern,
				})
			}
			required[name] = caps
			mu.Unlock()

			return nil
		})
	}

	// Wait for all plugins to load
	if err := g.Wait(); err != nil {
		return nil, err
	}

	return required, nil
}

// GrantCapabilities determines which capabilities to grant based on user input.
func (o *CapabilityOrchestrator) GrantCapabilities(required map[string][]capabilities.Capability) (map[string][]capabilities.Capability, error) {
	// Flatten all required capabilities to a unique set for user prompting
	flatRequired := capabilities.NewGrant()
	for _, caps := range required {
		for _, cap := range caps {
			flatRequired.Add(cap)
		}
	}

	// If trustAll flag is set, grant everything
	if o.trustAll {
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

		for _, cap := range missing {
			granted, always, err := o.prompter.PromptForCapability(cap)
			if err != nil {
				return nil, err
			}

			if granted {
				newGrants.Add(cap)
				if always {
					shouldSave = true
				}
			} else {
				return nil, fmt.Errorf("capability denied by user: %s", cap.String())
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
		for _, cap := range caps {
			if grantedGlobal.Contains(cap) {
				allowed.Add(cap)
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
	for _, cap := range required {
		if !granted.Contains(cap) {
			missing.Add(cap)
		}
	}
	return missing
}
