// Package ports defines interfaces for infrastructure dependencies.
// CapabilityInfo contains metadata about a capability request.
// This is placed in ports to avoid circular imports between services and ports.
package ports

import "github.com/reglet-dev/reglet/internal/domain/capabilities"

// CapabilityInfo contains metadata about a capability request.
type CapabilityInfo struct {
	Capability      capabilities.Capability
	IsProfileBased  bool                     // True if extracted from profile config
	PluginName      string                   // Which plugin requested this
	IsBroad         bool                     // True if pattern is overly permissive
	ProfileSpecific *capabilities.Capability // Profile-specific alternative if available
}
