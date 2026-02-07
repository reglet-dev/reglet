package services

import (
	"github.com/reglet-dev/reglet-host-sdk/capability"
	"github.com/reglet-dev/reglet-host-sdk/capability/gatekeeper"
	"github.com/reglet-dev/reglet-host-sdk/capability/grantstore"

	"github.com/reglet-dev/reglet/internal/application/ports"
	internalCap "github.com/reglet-dev/reglet/internal/domain/capability"
	infraCapabilities "github.com/reglet-dev/reglet/internal/infrastructure/capabilities"
)

// CapabilityGatekeeper wraps host-sdk's gatekeeper.Gatekeeper,
// converting between internal capability types and ABI types.
// This wrapper will be removed in Phase 04 when mirror types are eliminated.
type CapabilityGatekeeper struct {
	inner *gatekeeper.Gatekeeper
}

func NewCapabilityGatekeeper(configPath string, securityLevel string) *CapabilityGatekeeper {
	return &CapabilityGatekeeper{
		inner: gatekeeper.NewGatekeeper(
			gatekeeper.WithStore(grantstore.NewFileStore(grantstore.WithPath(configPath))),
			gatekeeper.WithSecurityLevel(gatekeeper.SecurityLevel(securityLevel)),
		),
	}
}

func (g *CapabilityGatekeeper) GrantCapabilities(
	required internalCap.GrantSet,
	capabilityInfo map[string]ports.CapabilityInfo,
	trustAll bool,
) (internalCap.GrantSet, error) {
	// Convert internal types to ABI types
	abiRequired := infraCapabilities.ToABI(required)

	// Convert CapabilityInfo
	info := make(map[string]capability.CapabilityInfo, len(capabilityInfo))
	for k, v := range capabilityInfo {
		info[k] = capability.CapabilityInfo{
			PluginName:     v.PluginName,
			IsProfileBased: v.IsProfileBased,
			IsBroad:        v.IsBroad,
		}
	}

	// Delegate to host-sdk
	abiGranted, err := g.inner.GrantCapabilities(abiRequired, info, trustAll)
	if err != nil {
		return internalCap.GrantSet{}, err
	}

	// Convert back to internal types
	return infraCapabilities.FromABI(abiGranted), nil
}
