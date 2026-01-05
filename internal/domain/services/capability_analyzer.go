package services

import (
	"github.com/whiskeyjimbo/reglet/internal/domain/capabilities"
	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
)

// CapabilityAnalyzer extracts specific capability requirements from profiles.
// This is a pure domain service with no infrastructure dependencies.
type CapabilityAnalyzer struct{}

// NewCapabilityAnalyzer creates a new capability analyzer.
func NewCapabilityAnalyzer() *CapabilityAnalyzer {
	return &CapabilityAnalyzer{}
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

			// Extract plugin-specific capabilities based on config
			extractedCaps := a.extractFromObservation(pluginName, obs.Config)

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

// extractFromObservation extracts capabilities from a single observation config.
// This is plugin-specific logic that understands each plugin's config schema.
func (a *CapabilityAnalyzer) extractFromObservation(pluginName string, config map[string]interface{}) []capabilities.Capability {
	var extractedCaps []capabilities.Capability

	switch pluginName {
	case "file":
		// Extract file path from config
		if pathVal, ok := config["path"]; ok {
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
		if cmdVal, ok := config["command"]; ok {
			if cmd, ok := cmdVal.(string); ok && cmd != "" {
				extractedCaps = append(extractedCaps, capabilities.Capability{
					Kind:    "exec",
					Pattern: cmd,
				})
			}
		}

	case "http", "tcp", "dns":
		// Network plugins - extract specific endpoints if available
		if urlVal, ok := config["url"]; ok {
			if url, ok := urlVal.(string); ok && url != "" {
				extractedCaps = append(extractedCaps, capabilities.Capability{
					Kind:    "network",
					Pattern: "outbound:" + url,
				})
			}
		} else if hostVal, ok := config["host"]; ok {
			if host, ok := hostVal.(string); ok && host != "" {
				extractedCaps = append(extractedCaps, capabilities.Capability{
					Kind:    "network",
					Pattern: "outbound:" + host,
				})
			}
		}
	}

	return extractedCaps
}
