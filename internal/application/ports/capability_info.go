// Package ports defines interfaces for infrastructure dependencies.
// CapabilityInfo contains metadata about a capability request.
// This is placed in ports to avoid circular imports between services and ports.
package ports

import "github.com/reglet-dev/reglet/internal/domain/capabilities"

// CapabilityInfo contains metadata about a capability request.
type CapabilityInfo struct {
	ProfileSpecific *capabilities.Capability
	Capability      capabilities.Capability
	PluginName      string
	IsProfileBased  bool
	IsBroad         bool
}
