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
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/build"
	infraCapabilities "github.com/whiskeyjimbo/reglet/internal/infrastructure/capabilities"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/wasm"
	"golang.org/x/sync/errgroup"
)

// CapabilityOrchestrator manages capability collection and granting.
// Coordinates domain and infrastructure.
type CapabilityOrchestrator struct {
	fileStore *infraCapabilities.FileStore
	prompter  *infraCapabilities.TerminalPrompter
	grants    capabilities.Grant
	trustAll  bool // Auto-grant all capabilities
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
			for _, capability := range info.Capabilities {
				caps = append(caps, capabilities.Capability{
					Kind:    capability.Kind,
					Pattern: capability.Pattern,
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
			granted, always, err := o.prompter.PromptForCapability(capability)
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
