package services

import (
	"log/slog"

	sdkEntities "github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/capabilities"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/pkg/loopexpander"
)

// CapabilityAnalyzer extracts specific capability requirements from profiles.
// This is a pure domain service with no infrastructure dependencies.
type CapabilityAnalyzer struct {
	registry *capabilities.Registry
}

// NewCapabilityAnalyzer creates a new capability analyzer.
func NewCapabilityAnalyzer(registry *capabilities.Registry) *CapabilityAnalyzer {
	return &CapabilityAnalyzer{
		registry: registry,
	}
}

// ExtractCapabilities analyzes profile observations to extract specific capability requirements.
// This enables principle of least privilege by requesting only the resources actually used,
// rather than the plugin's full declared capabilities.
//
// Returns a map of plugin name to required GrantSet.
func (a *CapabilityAnalyzer) ExtractCapabilities(profile entities.ProfileReader) map[string]*sdkEntities.GrantSet {
	// Delegate to ExtractCapabilitiesWithVars with the profile's vars
	return a.ExtractCapabilitiesWithVars(profile, profile.GetVars())
}

// ExtractCapabilitiesWithVars analyzes profile observations to extract specific capability requirements,
// expanding loop observations using the provided vars to extract specific paths.
//
// This is the security-first implementation that ensures loop observations request only
// the specific resources they will access, rather than falling back to broad wildcards.
func (a *CapabilityAnalyzer) ExtractCapabilitiesWithVars(profile entities.ProfileReader, vars map[string]interface{}) map[string]*sdkEntities.GrantSet {
	// Accumulate GrantSets per plugin
	profileCaps := make(map[string]*sdkEntities.GrantSet)

	// Analyze each control's observations
	for _, ctrl := range profile.GetControls() {
		for _, obs := range ctrl.ObservationDefinitions {
			pluginName := obs.Plugin

			// Initialize plugin entry if needed
			if _, ok := profileCaps[pluginName]; !ok {
				profileCaps[pluginName] = &sdkEntities.GrantSet{}
			}

			// Look up extractor for this plugin
			extractor, ok := a.registry.Get(pluginName)
			if !ok {
				// No specific extractor found. Assume no additional dynamic capabilities are needed
				// beyond what the plugin declares in its manifest.
				continue
			}

			// Check if this is a loop observation - expand it before extracting capabilities
			if obs.Loop != nil && vars != nil {
				a.extractLoopCapabilities(obs, vars, extractor, profileCaps[pluginName])
			} else {
				// Regular observation - extract directly
				extractedCaps := extractor.Extract(obs.Config)
				if extractedCaps != nil {
					profileCaps[pluginName].Merge(extractedCaps)
				}
			}
		}
	}

	// Remove empty GrantSets
	result := make(map[string]*sdkEntities.GrantSet)
	for pluginName, gs := range profileCaps {
		if gs != nil && !gs.IsEmpty() {
			result[pluginName] = gs
		}
	}

	return result
}

// extractLoopCapabilities expands a loop observation and extracts capabilities from each expanded config.
func (a *CapabilityAnalyzer) extractLoopCapabilities(
	obs entities.ObservationDefinition,
	vars map[string]interface{},
	extractor capabilities.Extractor,
	grantSet *sdkEntities.GrantSet,
) {
	// Resolve the loop items from vars
	items, err := loopexpander.ResolveLoopItems(obs.Loop.Items, vars)
	if err != nil {
		slog.Warn("failed to resolve loop items for capability extraction",
			"items_expr", obs.Loop.Items,
			"error", err)
		return
	}

	if len(items) == 0 {
		return
	}

	// Extract capabilities from each expanded config
	customName := obs.Loop.As
	for i, item := range items {
		loopCtx := &loopexpander.Context{
			Item:   item,
			Index:  i,
			First:  i == 0,
			Last:   i == len(items)-1,
			Length: len(items),
		}

		// Substitute loop variables into the config
		expandedConfig := loopexpander.SubstituteLoopInMap(obs.Config, loopCtx, customName)

		// Extract capabilities from the expanded config
		extractedCaps := extractor.Extract(expandedConfig)
		if extractedCaps != nil {
			grantSet.Merge(extractedCaps)
		}
	}
}
