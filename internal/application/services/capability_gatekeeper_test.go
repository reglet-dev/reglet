package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/whiskeyjimbo/reglet/internal/application/ports"
	"github.com/whiskeyjimbo/reglet/internal/domain/capabilities"
)

func TestCapabilityGatekeeper_TrustAllMode(t *testing.T) {
	gatekeeper := NewCapabilityGatekeeper("/tmp/test-config.yaml", "standard")

	required := capabilities.NewGrant()
	required.Add(capabilities.Capability{Kind: "fs", Pattern: "read:/etc/passwd"})
	required.Add(capabilities.Capability{Kind: "exec", Pattern: "/bin/ls"})

	capInfo := make(map[string]ports.CapabilityInfo)

	// Trust all mode should grant everything without prompting
	granted, err := gatekeeper.GrantCapabilities(required, capInfo, true)

	require.NoError(t, err)
	assert.Len(t, granted, 2)
	assert.True(t, granted.Contains(capabilities.Capability{Kind: "fs", Pattern: "read:/etc/passwd"}))
	assert.True(t, granted.Contains(capabilities.Capability{Kind: "exec", Pattern: "/bin/ls"}))
}

func TestCapabilityGatekeeper_FindMissingCapabilities(t *testing.T) {
	gatekeeper := NewCapabilityGatekeeper("/tmp/test-config.yaml", "standard")

	required := capabilities.NewGrant()
	required.Add(capabilities.Capability{Kind: "fs", Pattern: "read:/etc/passwd"})
	required.Add(capabilities.Capability{Kind: "fs", Pattern: "read:/etc/shadow"})
	required.Add(capabilities.Capability{Kind: "exec", Pattern: "/bin/ls"})

	existing := capabilities.NewGrant()
	existing.Add(capabilities.Capability{Kind: "fs", Pattern: "read:/etc/passwd"}) // Already granted

	missing := gatekeeper.findMissingCapabilities(required, existing)

	assert.Len(t, missing, 2)
	assert.True(t, missing.Contains(capabilities.Capability{Kind: "fs", Pattern: "read:/etc/shadow"}))
	assert.True(t, missing.Contains(capabilities.Capability{Kind: "exec", Pattern: "/bin/ls"}))
	assert.False(t, missing.Contains(capabilities.Capability{Kind: "fs", Pattern: "read:/etc/passwd"}))
}

func TestCapabilityGatekeeper_SecurityLevels(t *testing.T) {
	tests := []struct {
		name          string
		securityLevel string
		capability    capabilities.Capability
		isBroad       bool
		expectDenied  bool // true if strict mode should deny
	}{
		{
			name:          "Strict denies broad capabilities",
			securityLevel: "strict",
			capability:    capabilities.Capability{Kind: "fs", Pattern: "read:**"},
			isBroad:       true,
			expectDenied:  true,
		},
		{
			name:          "Standard allows non-broad (would prompt in real scenario)",
			securityLevel: "standard",
			capability:    capabilities.Capability{Kind: "fs", Pattern: "read:/etc/passwd"},
			isBroad:       false,
			expectDenied:  false,
		},
		{
			name:          "Permissive in trust-all mode",
			securityLevel: "permissive",
			capability:    capabilities.Capability{Kind: "fs", Pattern: "read:**"},
			isBroad:       true,
			expectDenied:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gatekeeper := NewCapabilityGatekeeper("/tmp/test-config.yaml", tt.securityLevel)

			required := capabilities.NewGrant()
			required.Add(tt.capability)

			capInfo := make(map[string]ports.CapabilityInfo)
			if tt.isBroad {
				key := tt.capability.Kind + ":" + tt.capability.Pattern
				capInfo[key] = ports.CapabilityInfo{
					Capability: tt.capability,
					IsBroad:    true,
					PluginName: "test",
				}
			}

			// For strict mode with broad capabilities, we expect an error
			if tt.expectDenied && tt.securityLevel == "strict" {
				_, err := gatekeeper.GrantCapabilities(required, capInfo, false)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "denied by strict security policy")
				return
			}

			// For permissive mode or trust-all, should succeed
			if tt.securityLevel == "permissive" {
				granted, err := gatekeeper.GrantCapabilities(required, capInfo, false)
				require.NoError(t, err)
				assert.True(t, granted.Contains(tt.capability))
			}
		})
	}
}

func TestCapabilityGatekeeper_EmptyRequired(t *testing.T) {
	gatekeeper := NewCapabilityGatekeeper("/tmp/test-config.yaml", "standard")

	required := capabilities.NewGrant() // Empty
	capInfo := make(map[string]ports.CapabilityInfo)

	granted, err := gatekeeper.GrantCapabilities(required, capInfo, false)

	require.NoError(t, err)
	assert.Empty(t, granted)
}
