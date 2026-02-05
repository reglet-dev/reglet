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

	sdkEntities "github.com/reglet-dev/reglet-sdk/domain/entities"
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
func (o *CapabilityOrchestrator) CollectCapabilities(ctx context.Context, profile entities.ProfileReader, pluginDir string) (map[string]*sdkEntities.GrantSet, ports.PluginRuntime, error) {
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
func (o *CapabilityOrchestrator) CollectRequiredCapabilities(ctx context.Context, profile entities.ProfileReader, runtime ports.PluginRuntime, pluginDir string) (map[string]*sdkEntities.GrantSet, error) {
	// Extract specific capabilities from profile observation configs (returns GrantSet)
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
func (o *CapabilityOrchestrator) loadPluginCapabilities(ctx context.Context, runtime ports.PluginRuntime, pluginDir string, pluginNames map[string]bool) (map[string]*sdkEntities.GrantSet, error) {
	// Convert to slice for parallel iteration
	names := make([]string, 0, len(pluginNames))
	for name := range pluginNames {
		names = append(names, name)
	}

	// Thread-safe map for collecting plugin metadata capabilities
	var mu sync.Mutex
	pluginMetaCaps := make(map[string]*sdkEntities.GrantSet)

	g, gctx := errgroup.WithContext(ctx)
	for _, name := range names {
		g.Go(func() error {
			gs, err := o.loadSinglePlugin(gctx, runtime, pluginDir, name)
			if err != nil {
				return err
			}

			mu.Lock()
			pluginMetaCaps[name] = gs
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return pluginMetaCaps, nil
}

// loadSinglePlugin loads a single plugin and returns its declared capabilities as GrantSet.
func (o *CapabilityOrchestrator) loadSinglePlugin(ctx context.Context, runtime ports.PluginRuntime, pluginDir, name string) (*sdkEntities.GrantSet, error) {
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
	manifest, err := plugin.Manifest(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get capabilities from plugin %s: %w", name, err)
	}

	// Extract capabilities into GrantSet for gatekeeper
	// Manifest now directly contains GrantSet (no conversion needed)
	return &manifest.Capabilities, nil
}

// mergeCapabilities merges profile-extracted capabilities with plugin metadata.
// Profile-extracted capabilities take precedence (more specific).
func (o *CapabilityOrchestrator) mergeCapabilities(pluginNames map[string]bool, profileCaps, pluginMetaCaps map[string]*sdkEntities.GrantSet) (map[string]*sdkEntities.GrantSet, error) {
	required := make(map[string]*sdkEntities.GrantSet)

	// Clear and rebuild capability info metadata
	o.capabilityInfo = make(map[string]ports.CapabilityInfo)

	for name := range pluginNames {
		profileGS := profileCaps[name]
		metaGS := pluginMetaCaps[name]

		if profileGS != nil && !profileGS.IsEmpty() {
			required[name] = profileGS
			o.recordCapabilityInfo(name, profileGS, true)
			slog.Debug("using profile-extracted capabilities",
				"plugin", name,
				"capabilities", profileGS)
		} else if metaGS != nil && !metaGS.IsEmpty() {
			required[name] = metaGS
			o.recordCapabilityInfo(name, metaGS, false)
			slog.Debug("using plugin metadata capabilities (fallback)",
				"plugin", name,
				"capabilities", metaGS)
		}
	}

	return required, nil
}

// recordCapabilityInfo records capability metadata for the gatekeeper.
func (o *CapabilityOrchestrator) recordCapabilityInfo(name string, gs *sdkEntities.GrantSet, isProfileBased bool) {
	// Record network capabilities
	if gs.Network != nil {
		for _, rule := range gs.Network.Rules {
			key := fmt.Sprintf("network:%v:%v", rule.Hosts, rule.Ports)
			o.capabilityInfo[key] = ports.CapabilityInfo{
				IsProfileBased: isProfileBased,
				PluginName:     name,
				IsBroad:        len(rule.Hosts) == 1 && rule.Hosts[0] == "*" && len(rule.Ports) == 1 && rule.Ports[0] == "*",
			}
		}
	}

	// Record filesystem capabilities
	if gs.FS != nil {
		for _, rule := range gs.FS.Rules {
			for _, path := range rule.Read {
				key := "fs:read:" + path
				o.capabilityInfo[key] = ports.CapabilityInfo{
					IsProfileBased: isProfileBased,
					PluginName:     name,
					IsBroad:        path == "/**" || path == "**",
				}
			}
			for _, path := range rule.Write {
				key := "fs:write:" + path
				o.capabilityInfo[key] = ports.CapabilityInfo{
					IsProfileBased: isProfileBased,
					PluginName:     name,
					IsBroad:        path == "/**" || path == "**",
				}
			}
		}
	}

	// Record environment capabilities
	if gs.Env != nil {
		for _, v := range gs.Env.Variables {
			key := "env:" + v
			o.capabilityInfo[key] = ports.CapabilityInfo{
				IsProfileBased: isProfileBased,
				PluginName:     name,
				IsBroad:        v == "*",
			}
		}
	}

	// Record exec capabilities
	if gs.Exec != nil {
		for _, cmd := range gs.Exec.Commands {
			key := "exec:" + cmd
			o.capabilityInfo[key] = ports.CapabilityInfo{
				IsProfileBased: isProfileBased,
				PluginName:     name,
				IsBroad:        cmd == "**" || cmd == "*",
			}
		}
	}
}

// GrantCapabilities resolves permissions via the gatekeeper.
// Delegates the complete granting workflow to CapabilityGatekeeper.
func (o *CapabilityOrchestrator) GrantCapabilities(required map[string]*sdkEntities.GrantSet, trustAll bool) (map[string]*sdkEntities.GrantSet, error) {
	// Flatten all required capabilities into a single GrantSet
	flatRequired := &sdkEntities.GrantSet{}
	for _, gs := range required {
		if gs != nil {
			flatRequired.Merge(gs)
		}
	}

	// Delegate granting decision to the gatekeeper
	grantedGlobal, err := o.gatekeeper.GrantCapabilities(flatRequired, o.capabilityInfo, o.trustAll || trustAll)
	if err != nil {
		return nil, err
	}

	// For now, return the original required capabilities if granted
	// The gatekeeper has validated that these are allowed
	if grantedGlobal != nil && !grantedGlobal.IsEmpty() {
		return required, nil
	}

	return nil, nil
}
