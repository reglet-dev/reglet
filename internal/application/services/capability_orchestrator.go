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
	domainServices "github.com/whiskeyjimbo/reglet/internal/domain/services"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/build"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/wasm"
	"golang.org/x/sync/errgroup"
)

// CapabilityInfo contains metadata about a capability request.
type CapabilityInfo struct {
	Capability      capabilities.Capability
	IsProfileBased  bool                     // True if extracted from profile config
	PluginName      string                   // Which plugin requested this
	IsBroad         bool                     // True if pattern is overly permissive
	ProfileSpecific *capabilities.Capability // Profile-specific alternative if available
}

// CapabilityOrchestrator coordinates capability collection and granting.
// It delegates to specialized services:
// - CapabilityAnalyzer for extraction (domain logic)
// - CapabilityGatekeeper for granting (security boundary)
type CapabilityOrchestrator struct {
	analyzer       *domainServices.CapabilityAnalyzer // Domain service for extraction
	gatekeeper     *CapabilityGatekeeper              // Application service for granting
	trustAll       bool                               // Auto-grant all capabilities
	capabilityInfo map[string]CapabilityInfo          // Metadata about requested capabilities
}

// NewCapabilityOrchestrator creates a capability orchestrator with default security level (standard).
func NewCapabilityOrchestrator(trustAll bool, registry *capabilities.Registry) *CapabilityOrchestrator {
	return NewCapabilityOrchestratorWithSecurity(trustAll, "standard", registry)
}

// NewCapabilityOrchestratorWithSecurity creates a capability orchestrator with specified security level.
// securityLevel can be: "strict", "standard", or "permissive"
func NewCapabilityOrchestratorWithSecurity(trustAll bool, securityLevel string, registry *capabilities.Registry) *CapabilityOrchestrator {
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".reglet", "config.yaml")

	return &CapabilityOrchestrator{
		analyzer:       domainServices.NewCapabilityAnalyzer(registry),
		gatekeeper:     NewCapabilityGatekeeper(configPath, securityLevel),
		trustAll:       trustAll,
		capabilityInfo: make(map[string]CapabilityInfo),
	}
}

// CollectCapabilities creates a temporary runtime and collects required capabilities.
// Returns the required capabilities and the temporary runtime (caller must close it).
func (o *CapabilityOrchestrator) CollectCapabilities(ctx context.Context, profile entities.ProfileReader, pluginDir string) (map[string][]capabilities.Capability, *wasm.Runtime, error) {
	// Create temporary runtime for capability collection
	runtime, err := wasm.NewRuntime(ctx, build.Get())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temporary runtime: %w", err)
	}

	caps, err := o.CollectRequiredCapabilities(ctx, profile, runtime, pluginDir)
	if err != nil {
		if closeErr := runtime.Close(ctx); closeErr != nil {
			slog.ErrorContext(ctx, "failed to close temporary runtime", "error", closeErr)
		}
		return nil, nil, err
	}

	return caps, runtime, nil
}

// CollectRequiredCapabilities loads plugins and identifies requirements.
// It prioritizes specific capabilities extracted from profile configs over plugin metadata.
func (o *CapabilityOrchestrator) CollectRequiredCapabilities(ctx context.Context, profile entities.ProfileReader, runtime *wasm.Runtime, pluginDir string) (map[string][]capabilities.Capability, error) {
	// First, extract specific capabilities from profile observation configs using domain service
	profileCaps := o.analyzer.ExtractCapabilities(profile)

	// Get unique plugin names from profile
	pluginNames := make(map[string]bool)
	for _, ctrl := range profile.GetAllControls() {
		for _, obs := range ctrl.ObservationDefinitions {
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
		g.Go(func() error {
			// Security: Validate plugin name to prevent path traversal
			if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
				return fmt.Errorf("invalid plugin name %q: contains path separator or traversal", name)
			}

			// SECURITY: Use os.OpenRoot to prevent symlink-based path traversal.
			// This ensures plugins cannot escape the plugin directory via symlinks.
			rootDir, err := os.OpenRoot(pluginDir)
			if err != nil {
				return fmt.Errorf("failed to open plugin directory %s: %w", pluginDir, err)
			}
			defer func() {
				_ = rootDir.Close() // Best-effort cleanup
			}()

			// Read plugin file using sandboxed Root.ReadFile (Go 1.25+)
			pluginSubpath := filepath.Join(name, name+".wasm")
			wasmBytes, err := rootDir.ReadFile(pluginSubpath)
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
			for _, capability := range profileSpecific {
				key := capability.Kind + ":" + capability.Pattern
				o.capabilityInfo[key] = CapabilityInfo{
					Capability:      capability,
					IsProfileBased:  true,
					PluginName:      name,
					IsBroad:         capability.IsBroad(),
					ProfileSpecific: nil,
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
			for _, capability := range metaCaps {
				key := capability.Kind + ":" + capability.Pattern
				info := CapabilityInfo{
					Capability:     capability,
					IsProfileBased: false,
					PluginName:     name,
					IsBroad:        capability.IsBroad(),
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

// GrantCapabilities resolves permissions via the gatekeeper.
// Delegates the complete granting workflow to CapabilityGatekeeper.
func (o *CapabilityOrchestrator) GrantCapabilities(required map[string][]capabilities.Capability, trustAll bool) (map[string][]capabilities.Capability, error) {
	// Flatten all required capabilities to a unique set
	flatRequired := capabilities.NewGrant()
	for _, caps := range required {
		for _, capability := range caps {
			flatRequired.Add(capability)
		}
	}

	// Delegate granting decision to the gatekeeper
	grantedGlobal, err := o.gatekeeper.GrantCapabilities(flatRequired, o.capabilityInfo, o.trustAll || trustAll)
	if err != nil {
		return nil, err
	}

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
