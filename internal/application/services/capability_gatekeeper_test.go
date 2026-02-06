package services

import (
	"testing"

	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/domain/capability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapabilityGatekeeper_TrustAllMode(t *testing.T) {
	gatekeeper := NewCapabilityGatekeeper("/tmp/test-config.yaml", "standard")

	required := capability.GrantSet{
		FS:   &capability.FileSystemCapability{Rules: []capability.FileSystemRule{{Read: []string{"/etc/passwd"}}}},
		Exec: &capability.ExecCapability{Commands: []string{"/bin/ls"}},
	}

	capInfo := make(map[string]ports.CapabilityInfo)

	// Trust all mode should grant everything without prompting
	granted, err := gatekeeper.GrantCapabilities(required, capInfo, true)

	require.NoError(t, err)
	// granted is a value, not pointer
	assert.NotNil(t, granted.FS)
	assert.NotNil(t, granted.Exec)
}

func TestCapabilityGatekeeper_FindMissingCapabilities(t *testing.T) {
	required := capability.GrantSet{
		FS:   &capability.FileSystemCapability{Rules: []capability.FileSystemRule{{Read: []string{"/etc/passwd", "/etc/shadow"}}}},
		Exec: &capability.ExecCapability{Commands: []string{"/bin/ls"}},
	}

	existing := capability.GrantSet{
		FS: &capability.FileSystemCapability{Rules: []capability.FileSystemRule{{Read: []string{"/etc/passwd"}}}},
	}

	missing := required.Difference(&existing)

	// Missing should contain /etc/shadow (not /etc/passwd which is already granted) and /bin/ls
	assert.NotNil(t, missing.FS)
	assert.NotNil(t, missing.Exec)
}

func TestCapabilityGatekeeper_SecurityLevels(t *testing.T) {
	tests := []struct {
		name          string
		securityLevel string
		required      capability.GrantSet
		isBroad       bool
		expectDenied  bool // true if strict mode should deny
	}{
		{
			name:          "Strict denies broad capabilities",
			securityLevel: "strict",
			required:      capability.GrantSet{FS: &capability.FileSystemCapability{Rules: []capability.FileSystemRule{{Read: []string{"**"}}}}},
			isBroad:       true,
			expectDenied:  true,
		},
		{
			name:          "Standard allows non-broad (would prompt in real scenario)",
			securityLevel: "standard",
			required:      capability.GrantSet{FS: &capability.FileSystemCapability{Rules: []capability.FileSystemRule{{Read: []string{"/etc/passwd"}}}}},
			isBroad:       false,
			expectDenied:  false,
		},
		{
			name:          "Permissive in trust-all mode",
			securityLevel: "permissive",
			required:      capability.GrantSet{FS: &capability.FileSystemCapability{Rules: []capability.FileSystemRule{{Read: []string{"**"}}}}},
			isBroad:       true,
			expectDenied:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gatekeeper := NewCapabilityGatekeeper("/tmp/test-config.yaml", tt.securityLevel)

			capInfo := make(map[string]ports.CapabilityInfo)
			if tt.isBroad {
				capInfo["fs:read:**"] = ports.CapabilityInfo{
					IsBroad:    true,
					PluginName: "test",
				}
			}

			// For strict mode with broad capabilities, we expect an error
			if tt.expectDenied && tt.securityLevel == "strict" {
				_, err := gatekeeper.GrantCapabilities(tt.required, capInfo, false)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "denied by strict security policy")
				return
			}

			// For permissive mode or trust-all, should succeed
			if tt.securityLevel == "permissive" {
				granted, err := gatekeeper.GrantCapabilities(tt.required, capInfo, false)
				require.NoError(t, err)
				// granted is value
				assert.False(t, granted.IsEmpty())
			}
		})
	}
}

func TestCapabilityGatekeeper_EmptyRequired(t *testing.T) {
	gatekeeper := NewCapabilityGatekeeper("/tmp/test-config.yaml", "standard")

	required := capability.GrantSet{} // Empty
	capInfo := make(map[string]ports.CapabilityInfo)

	granted, err := gatekeeper.GrantCapabilities(required, capInfo, false)

	require.NoError(t, err)
	assert.True(t, granted.IsEmpty())
}
