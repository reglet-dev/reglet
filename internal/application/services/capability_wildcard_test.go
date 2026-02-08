package services

import (
	"testing"

	"github.com/reglet-dev/reglet-abi/hostfunc"
	"github.com/reglet-dev/reglet-host-sdk/capability"
	"github.com/stretchr/testify/assert"
)

// TestRiskAssessor_ExecWildcards verifies that exec capabilities are detected as critical risk.
// Note: Internal AnalyzeRisk marks ALL exec as critical
func TestRiskAssessor_ExecWildcards(t *testing.T) {
	tests := []struct {
		name        string
		grantSet    *hostfunc.GrantSet
		expectLevel capability.RiskLevel
	}{
		{
			name: "exec:** is critical risk (arbitrary command execution)",
			grantSet: &hostfunc.GrantSet{
				Exec: &hostfunc.ExecCapability{Commands: []string{"**"}},
			},
			expectLevel: capability.RiskCritical,
		},
		{
			name: "exec:* is critical risk",
			grantSet: &hostfunc.GrantSet{
				Exec: &hostfunc.ExecCapability{Commands: []string{"*"}},
			},
			expectLevel: capability.RiskCritical,
		},
		{
			name: "specific binary is also critical",
			grantSet: &hostfunc.GrantSet{
				Exec: &hostfunc.ExecCapability{Commands: []string{"/usr/bin/ls"}},
			},
			expectLevel: capability.RiskCritical,
		},
		{
			name: "shells are critical risk (allows arbitrary commands)",
			grantSet: &hostfunc.GrantSet{
				Exec: &hostfunc.ExecCapability{Commands: []string{"/bin/sh"}},
			},
			expectLevel: capability.RiskCritical,
		},
		{
			name: "interpreters are critical risk (allows code execution via -c)",
			grantSet: &hostfunc.GrantSet{
				Exec: &hostfunc.ExecCapability{Commands: []string{"python"}},
			},
			expectLevel: capability.RiskCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := capability.AnalyzeRisk(tt.grantSet)
			assert.Equal(t, tt.expectLevel, result.Level,
				"GrantSet should have expected risk level")
		})
	}
}

// TestRiskAssessor_FilesystemWildcards verifies that fs wildcard patterns are detected.
func TestRiskAssessor_FilesystemWildcards(t *testing.T) {
	tests := []struct {
		name        string
		grantSet    *hostfunc.GrantSet
		expectLevel capability.RiskLevel
	}{
		{
			name: "fs:read:** is medium risk",
			grantSet: &hostfunc.GrantSet{
				FS: &hostfunc.FileSystemCapability{
					Rules: []hostfunc.FileSystemRule{{Read: []string{"**"}}},
				},
			},
			expectLevel: capability.RiskMedium,
		},
		{
			name: "fs:write:** is high risk",
			grantSet: &hostfunc.GrantSet{
				FS: &hostfunc.FileSystemCapability{
					Rules: []hostfunc.FileSystemRule{{Write: []string{"**"}}},
				},
			},
			expectLevel: capability.RiskHigh,
		},
		{
			name: "root filesystem is medium risk (read)",
			grantSet: &hostfunc.GrantSet{
				FS: &hostfunc.FileSystemCapability{
					Rules: []hostfunc.FileSystemRule{{Read: []string{"/**"}}},
				},
			},
			expectLevel: capability.RiskMedium,
		},
		{
			name: "specific file is medium risk (read)",
			grantSet: &hostfunc.GrantSet{
				FS: &hostfunc.FileSystemCapability{
					Rules: []hostfunc.FileSystemRule{{Read: []string{"/etc/passwd"}}},
				},
			},
			expectLevel: capability.RiskMedium,
		},
		{
			name: "directory tree with ** is medium risk (read)",
			grantSet: &hostfunc.GrantSet{
				FS: &hostfunc.FileSystemCapability{
					Rules: []hostfunc.FileSystemRule{{Read: []string{"/var/log/**"}}},
				},
			},
			expectLevel: capability.RiskMedium,
		},
		{
			name: "/etc/** is medium risk (read - sensitive system config)",
			grantSet: &hostfunc.GrantSet{
				FS: &hostfunc.FileSystemCapability{
					Rules: []hostfunc.FileSystemRule{{Read: []string{"/etc/**"}}},
				},
			},
			expectLevel: capability.RiskMedium,
		},
		{
			name: "/home/** is medium risk (read - all user data)",
			grantSet: &hostfunc.GrantSet{
				FS: &hostfunc.FileSystemCapability{
					Rules: []hostfunc.FileSystemRule{{Read: []string{"/home/**"}}},
				},
			},
			expectLevel: capability.RiskMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := capability.AnalyzeRisk(tt.grantSet)
			assert.Equal(t, tt.expectLevel, result.Level,
				"GrantSet should have expected risk level")
		})
	}
}

// TestRiskAssessor_EnvironmentWildcards verifies env wildcard detection.
func TestRiskAssessor_EnvironmentWildcards(t *testing.T) {
	tests := []struct {
		name        string
		grantSet    *hostfunc.GrantSet
		expectLevel capability.RiskLevel
	}{
		{
			name: "env:* is low risk",
			grantSet: &hostfunc.GrantSet{
				Env: &hostfunc.EnvironmentCapability{Variables: []string{"*"}},
			},
			expectLevel: capability.RiskLow,
		},
		{
			name: "AWS_* is low risk",
			grantSet: &hostfunc.GrantSet{
				Env: &hostfunc.EnvironmentCapability{Variables: []string{"AWS_*"}},
			},
			expectLevel: capability.RiskLow,
		},
		{
			name: "specific variable is low risk",
			grantSet: &hostfunc.GrantSet{
				Env: &hostfunc.EnvironmentCapability{Variables: []string{"AWS_REGION"}},
			},
			expectLevel: capability.RiskLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := capability.AnalyzeRisk(tt.grantSet)
			assert.Equal(t, tt.expectLevel, result.Level,
				"GrantSet should have expected risk level")
		})
	}
}

// TestRiskAssessor_NetworkWildcards verifies network wildcard detection.
func TestRiskAssessor_NetworkWildcards(t *testing.T) {
	tests := []struct {
		name        string
		grantSet    *hostfunc.GrantSet
		expectLevel capability.RiskLevel
	}{
		{
			name: "network:* host is critical risk",
			grantSet: &hostfunc.GrantSet{
				Network: &hostfunc.NetworkCapability{
					Rules: []hostfunc.NetworkRule{{Hosts: []string{"*"}, Ports: []string{"443"}}},
				},
			},
			expectLevel: capability.RiskCritical,
		},
		{
			name: "specific host is medium risk",
			grantSet: &hostfunc.GrantSet{
				Network: &hostfunc.NetworkCapability{
					Rules: []hostfunc.NetworkRule{{Hosts: []string{"api.example.com"}, Ports: []string{"443"}}},
				},
			},
			expectLevel: capability.RiskMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := capability.AnalyzeRisk(tt.grantSet)
			assert.Equal(t, tt.expectLevel, result.Level,
				"GrantSet should have expected risk level")
		})
	}
}

// TestRiskAssessor_VersionedInterpreters verifies that interpreters are detected as critical risk.
func TestRiskAssessor_VersionedInterpreters(t *testing.T) {
	tests := []struct {
		name     string
		grantSet *hostfunc.GrantSet
	}{
		// Python versions
		{
			name: "python (base) is critical risk",
			grantSet: &hostfunc.GrantSet{
				Exec: &hostfunc.ExecCapability{Commands: []string{"python"}},
			},
		},
		{
			name: "python3 is critical risk",
			grantSet: &hostfunc.GrantSet{
				Exec: &hostfunc.ExecCapability{Commands: []string{"python3"}},
			},
		},
		{
			name: "python3.11 is critical risk (versioned)",
			grantSet: &hostfunc.GrantSet{
				Exec: &hostfunc.ExecCapability{Commands: []string{"python3.11"}},
			},
		},
		// Node.js versions
		{
			name: "node is critical risk",
			grantSet: &hostfunc.GrantSet{
				Exec: &hostfunc.ExecCapability{Commands: []string{"node"}},
			},
		},
		{
			name: "node18 is critical risk (versioned)",
			grantSet: &hostfunc.GrantSet{
				Exec: &hostfunc.ExecCapability{Commands: []string{"node18"}},
			},
		},
		// Ruby versions
		{
			name: "ruby is critical risk",
			grantSet: &hostfunc.GrantSet{
				Exec: &hostfunc.ExecCapability{Commands: []string{"ruby"}},
			},
		},
		// AWK variants
		{
			name: "awk is critical risk",
			grantSet: &hostfunc.GrantSet{
				Exec: &hostfunc.ExecCapability{Commands: []string{"awk"}},
			},
		},
		{
			name: "gawk is critical risk",
			grantSet: &hostfunc.GrantSet{
				Exec: &hostfunc.ExecCapability{Commands: []string{"gawk"}},
			},
		},
		// Note: All exec commands are critical risk
		{
			name: "specific command is also critical",
			grantSet: &hostfunc.GrantSet{
				Exec: &hostfunc.ExecCapability{Commands: []string{"systemctl"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := capability.AnalyzeRisk(tt.grantSet)
			assert.Equal(t, capability.RiskCritical, result.Level,
				"All exec should be critical risk")
		})
	}
}
