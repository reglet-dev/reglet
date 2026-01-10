package services

import (
	"github.com/reglet-dev/reglet/internal/domain/capabilities"
	"github.com/reglet-dev/reglet/internal/domain/entities"
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
// Returns a map of plugin name to required capabilities, deduplicated.
func (a *CapabilityAnalyzer) ExtractCapabilities(profile entities.ProfileReader) map[string][]capabilities.Capability {
	// Use map to deduplicate capabilities per plugin
	profileCaps := make(map[string]map[string]capabilities.Capability)

	// Analyze each control's observations
	for _, ctrl := range profile.GetAllControls() {
		for _, obs := range ctrl.ObservationDefinitions {
			pluginName := obs.Plugin

			// Initialize plugin entry if needed
			if _, ok := profileCaps[pluginName]; !ok {
				profileCaps[pluginName] = make(map[string]capabilities.Capability)
			}

			// Look up extractor for this plugin
			extractor, ok := a.registry.Get(pluginName)
			if !ok {
				// No specific extractor found. Assume no additional dynamic capabilities are needed
				// beyond what the plugin declares in its manifest.
				continue
			}

			// Extract plugin-specific capabilities based on config
			extractedCaps := extractor.Extract(obs.Config)

			// Deduplicate by using capability string as key
			for _, capability := range extractedCaps {
				key := capability.Kind + ":" + capability.Pattern
				profileCaps[pluginName][key] = capability
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
