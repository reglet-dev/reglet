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
	analyzer       ports.CapabilityAnalyzer
	gatekeeper     ports.CapabilityGatekeeperPort
	runtimeFactory ports.PluginRuntimeFactory
	capabilityInfo map[string]ports.CapabilityInfo
	trustAll       bool
}

// CapabilityOrchestratorOption configures a CapabilityOrchestrator.
type CapabilityOrchestratorOption func(*CapabilityOrchestrator)

// NewCapabilityOrchestrator creates a capability orchestrator with the given options.
// RuntimeFactory is required for creating plugin runtimes.
func NewCapabilityOrchestrator(
	runtimeFactory ports.PluginRuntimeFactory,
	opts ...CapabilityOrchestratorOption,
) *CapabilityOrchestrator {
	o := &CapabilityOrchestrator{
		runtimeFactory: runtimeFactory,
		capabilityInfo: make(map[string]ports.CapabilityInfo),
		// Defaults
		analyzer:   domainServices.NewCapabilityAnalyzer(capabilities.NewRegistry()),
		gatekeeper: NewCapabilityGatekeeper("", "standard"),
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// WithAnalyzer sets a custom capability analyzer.
func WithAnalyzer(a ports.CapabilityAnalyzer) CapabilityOrchestratorOption {
	return func(o *CapabilityOrchestrator) { o.analyzer = a }
}

// WithGatekeeper sets a custom capability gatekeeper.
func WithGatekeeper(g ports.CapabilityGatekeeperPort) CapabilityOrchestratorOption {
	return func(o *CapabilityOrchestrator) { o.gatekeeper = g }
}

// WithCapabilityRegistry sets a capability registry to use for the analyzer.
func WithCapabilityRegistry(r *capabilities.Registry) CapabilityOrchestratorOption {
	return func(o *CapabilityOrchestrator) {
		o.analyzer = domainServices.NewCapabilityAnalyzer(r)
	}
}

// WithSecurityConfig sets the config path and security level for the gatekeeper.
func WithSecurityConfig(configPath, securityLevel string) CapabilityOrchestratorOption {
	return func(o *CapabilityOrchestrator) {
		o.gatekeeper = NewCapabilityGatekeeper(configPath, securityLevel)
	}
}

// WithTrustAll sets the trust-all flag for capability granting.
func WithTrustAll(trust bool) CapabilityOrchestratorOption {
	return func(o *CapabilityOrchestrator) { o.trustAll = trust }
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
	for _, ctrl := range profile.GetControls() {
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
