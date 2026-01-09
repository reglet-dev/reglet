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

	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/domain/capabilities"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	domainServices "github.com/reglet-dev/reglet/internal/domain/services"
	"golang.org/x/sync/errgroup"
)

// CapabilityOrchestrator coordinates capability collection and granting.
// It delegates to specialized services:
// - CapabilityAnalyzer for extraction (domain logic)
// - CapabilityGatekeeper for granting (security boundary)
type CapabilityOrchestrator struct {
	analyzer       ports.CapabilityAnalyzer        // Domain service for extraction
	gatekeeper     ports.CapabilityGatekeeperPort  // Application service for granting
	runtimeFactory ports.PluginRuntimeFactory      // Factory for creating runtimes (injected)
	trustAll       bool                            // Auto-grant all capabilities
	capabilityInfo map[string]ports.CapabilityInfo // Metadata about requested capabilities
}

// NewCapabilityOrchestrator creates a capability orchestrator with default security level (standard).
// configPath specifies the path to the system config file (e.g., ~/.reglet/config.yaml).
func NewCapabilityOrchestrator(configPath string, trustAll bool, registry *capabilities.Registry, runtimeFactory ports.PluginRuntimeFactory) *CapabilityOrchestrator {
	return NewCapabilityOrchestratorWithSecurity(configPath, trustAll, "standard", registry, runtimeFactory)
}

// NewCapabilityOrchestratorWithSecurity creates a capability orchestrator with specified security level.
// configPath specifies the path to the system config file (e.g., ~/.reglet/config.yaml).
// securityLevel can be: "strict", "standard", or "permissive"
func NewCapabilityOrchestratorWithSecurity(configPath string, trustAll bool, securityLevel string, registry *capabilities.Registry, runtimeFactory ports.PluginRuntimeFactory) *CapabilityOrchestrator {
	return &CapabilityOrchestrator{
		analyzer:       domainServices.NewCapabilityAnalyzer(registry),
		gatekeeper:     NewCapabilityGatekeeper(configPath, securityLevel),
		runtimeFactory: runtimeFactory,
		trustAll:       trustAll,
		capabilityInfo: make(map[string]ports.CapabilityInfo),
	}
}

// NewCapabilityOrchestratorWithDeps creates an orchestrator with injected dependencies.
// This constructor is primarily for testing, allowing mock implementations.
func NewCapabilityOrchestratorWithDeps(
	analyzer ports.CapabilityAnalyzer,
	gatekeeper ports.CapabilityGatekeeperPort,
	runtimeFactory ports.PluginRuntimeFactory,
	trustAll bool,
) *CapabilityOrchestrator {
	return &CapabilityOrchestrator{
		analyzer:       analyzer,
		gatekeeper:     gatekeeper,
		runtimeFactory: runtimeFactory,
		trustAll:       trustAll,
		capabilityInfo: make(map[string]ports.CapabilityInfo),
	}
}

// CollectCapabilities creates a temporary runtime and collects required capabilities.
// Returns the required capabilities and the temporary runtime (caller must close it).
func (o *CapabilityOrchestrator) CollectCapabilities(ctx context.Context, profile entities.ProfileReader, pluginDir string) (map[string][]capabilities.Capability, ports.PluginRuntime, error) {
	// Create temporary runtime for capability collection
	runtime, err := o.runtimeFactory.NewRuntime(ctx)
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
func (o *CapabilityOrchestrator) CollectRequiredCapabilities(ctx context.Context, profile entities.ProfileReader, runtime ports.PluginRuntime, pluginDir string) (map[string][]capabilities.Capability, error) {
	// Extract specific capabilities from profile observation configs
	profileCaps := o.analyzer.ExtractCapabilities(profile)

	// Get unique plugin names from profile
	pluginNames := extractPluginNames(profile)

	// Load plugins in parallel to get their declared capabilities
	pluginMetaCaps, err := o.loadPluginCapabilities(ctx, runtime, pluginDir, pluginNames)
	if err != nil {
		return nil, err
	}

	// Merge profile-extracted capabilities with plugin metadata
	return o.mergeCapabilities(pluginNames, profileCaps, pluginMetaCaps)
}

// extractPluginNames gets unique plugin names from all profile controls.
func extractPluginNames(profile entities.ProfileReader) map[string]bool {
	pluginNames := make(map[string]bool)
	for _, ctrl := range profile.GetAllControls() {
		for _, obs := range ctrl.ObservationDefinitions {
			pluginNames[obs.Plugin] = true
		}
	}
	return pluginNames
}

// loadPluginCapabilities loads plugins in parallel and collects their declared capabilities.
func (o *CapabilityOrchestrator) loadPluginCapabilities(ctx context.Context, runtime ports.PluginRuntime, pluginDir string, pluginNames map[string]bool) (map[string][]capabilities.Capability, error) {
	// Convert to slice for parallel iteration
	names := make([]string, 0, len(pluginNames))
	for name := range pluginNames {
		names = append(names, name)
	}

	// Thread-safe map for collecting plugin metadata capabilities
	var mu sync.Mutex
	pluginMetaCaps := make(map[string][]capabilities.Capability)

	g, gctx := errgroup.WithContext(ctx)
	for _, name := range names {
		g.Go(func() error {
			caps, err := o.loadSinglePlugin(gctx, runtime, pluginDir, name)
			if err != nil {
				return err
			}

			mu.Lock()
			pluginMetaCaps[name] = caps
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return pluginMetaCaps, nil
}

// loadSinglePlugin loads a single plugin and returns its declared capabilities.
func (o *CapabilityOrchestrator) loadSinglePlugin(ctx context.Context, runtime ports.PluginRuntime, pluginDir, name string) ([]capabilities.Capability, error) {
	// Security: Validate plugin name to prevent path traversal
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return nil, fmt.Errorf("invalid plugin name %q: contains path separator or traversal", name)
	}

	// SECURITY: Use os.OpenRoot to prevent symlink-based path traversal.
	rootDir, err := os.OpenRoot(pluginDir)
	if err != nil {
		return nil, fmt.Errorf("failed to open plugin directory %s: %w", pluginDir, err)
	}
	defer func() { _ = rootDir.Close() }()

	// Read plugin file using sandboxed Root.ReadFile
	pluginSubpath := filepath.Join(name, name+".wasm")
	wasmBytes, err := rootDir.ReadFile(pluginSubpath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin %s: %w", name, err)
	}

	// Load plugin
	plugin, err := runtime.LoadPlugin(ctx, name, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to load plugin %s: %w", name, err)
	}

	// Get plugin metadata
	info, err := plugin.Describe(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get capabilities from plugin %s: %w", name, err)
	}

	// Convert to domain capabilities
	var caps []capabilities.Capability
	for _, capability := range info.Capabilities {
		caps = append(caps, capabilities.Capability{
			Kind:    capability.Kind,
			Pattern: capability.Pattern,
		})
	}

	return caps, nil
}

// mergeCapabilities merges profile-extracted capabilities with plugin metadata.
// Profile-extracted capabilities take precedence (more specific).
func (o *CapabilityOrchestrator) mergeCapabilities(pluginNames map[string]bool, profileCaps, pluginMetaCaps map[string][]capabilities.Capability) (map[string][]capabilities.Capability, error) {
	required := make(map[string][]capabilities.Capability)

	// Clear and rebuild capability info metadata
	o.capabilityInfo = make(map[string]ports.CapabilityInfo)

	for name := range pluginNames {
		profileSpecific := profileCaps[name]
		metaCaps := pluginMetaCaps[name]

		if len(profileSpecific) > 0 {
			o.useProfileCapabilities(name, profileSpecific, required)
		} else if len(metaCaps) > 0 {
			o.useMetadataCapabilities(name, metaCaps, profileSpecific, required)
		}
	}

	return required, nil
}

// useProfileCapabilities uses profile-extracted capabilities for a plugin.
func (o *CapabilityOrchestrator) useProfileCapabilities(name string, caps []capabilities.Capability, required map[string][]capabilities.Capability) {
	required[name] = caps
	slog.Debug("using profile-extracted capabilities",
		"plugin", name,
		"count", len(caps),
		"capabilities", caps)

	for _, capability := range caps {
		key := capability.Kind + ":" + capability.Pattern
		o.capabilityInfo[key] = ports.CapabilityInfo{
			Capability:      capability,
			IsProfileBased:  true,
			PluginName:      name,
			IsBroad:         capability.IsBroad(),
			ProfileSpecific: nil,
		}
	}
}

// useMetadataCapabilities uses plugin metadata capabilities as fallback.
func (o *CapabilityOrchestrator) useMetadataCapabilities(name string, metaCaps, profileCaps []capabilities.Capability, required map[string][]capabilities.Capability) {
	required[name] = metaCaps
	slog.Debug("using plugin metadata capabilities (fallback)",
		"plugin", name,
		"count", len(metaCaps),
		"capabilities", metaCaps)

	for _, capability := range metaCaps {
		key := capability.Kind + ":" + capability.Pattern
		info := ports.CapabilityInfo{
			Capability:     capability,
			IsProfileBased: false,
			PluginName:     name,
			IsBroad:        capability.IsBroad(),
		}

		// Check if there's a profile-specific alternative we could have used
		if len(profileCaps) > 0 {
			alt := profileCaps[0]
			info.ProfileSpecific = &alt
		}

		o.capabilityInfo[key] = info
	}
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
