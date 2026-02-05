package services

import (
	"testing"

	"github.com/reglet-dev/reglet-sdk/domain/entities"
	"github.com/stretchr/testify/assert"
)

// TestRiskAssessor_ExecWildcards verifies that exec capabilities are detected as critical risk.
// Note: SDK's SimpleRiskAnalyzer marks ALL exec as critical - it doesn't distinguish
func TestRiskAssessor_ExecWildcards(t *testing.T) {
	assessor := entities.NewSimpleRiskAnalyzer()

	tests := []struct {
		name        string
		grantSet    *entities.GrantSet
		expectLevel entities.RiskLevel
	}{
		{
			name: "exec:** is critical risk (arbitrary command execution)",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"**"}},
			},
			expectLevel: entities.RiskCritical,
		},
		{
			name: "exec:* is critical risk",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"*"}},
			},
			expectLevel: entities.RiskCritical,
		},
		{
			name: "specific binary is also critical (SDK doesn't distinguish)",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"/usr/bin/ls"}},
			},
			expectLevel: entities.RiskCritical, // SDK marks all exec as critical
		},
		{
			name: "shells are critical risk (allows arbitrary commands)",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"/bin/sh"}},
			},
			expectLevel: entities.RiskCritical,
		},
		{
			name: "interpreters are critical risk (allows code execution via -c)",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"python"}},
			},
			expectLevel: entities.RiskCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := assessor.Analyze(tt.grantSet)
			assert.Equal(t, tt.expectLevel, result.Level,
				"GrantSet should have expected risk level")
		})
	}
}

// TestRiskAssessor_FilesystemWildcards verifies that fs wildcard patterns are detected.
// Note: SDK's SimpleRiskAnalyzer marks write as RiskHigh and read as RiskMedium.
func TestRiskAssessor_FilesystemWildcards(t *testing.T) {
	assessor := entities.NewSimpleRiskAnalyzer()

	tests := []struct {
		name        string
		grantSet    *entities.GrantSet
		expectLevel entities.RiskLevel
	}{
		{
			name: "fs:read:** is medium risk",
			grantSet: &entities.GrantSet{
				FS: &entities.FileSystemCapability{
					Rules: []entities.FileSystemRule{{Read: []string{"**"}}},
				},
			},
			expectLevel: entities.RiskMedium,
		},
		{
			name: "fs:write:** is high risk",
			grantSet: &entities.GrantSet{
				FS: &entities.FileSystemCapability{
					Rules: []entities.FileSystemRule{{Write: []string{"**"}}},
				},
			},
			expectLevel: entities.RiskHigh,
		},
		{
			name: "root filesystem is medium risk (read)",
			grantSet: &entities.GrantSet{
				FS: &entities.FileSystemCapability{
					Rules: []entities.FileSystemRule{{Read: []string{"/**"}}},
				},
			},
			expectLevel: entities.RiskMedium,
		},
		{
			name: "specific file is medium risk (read)",
			grantSet: &entities.GrantSet{
				FS: &entities.FileSystemCapability{
					Rules: []entities.FileSystemRule{{Read: []string{"/etc/passwd"}}},
				},
			},
			expectLevel: entities.RiskMedium,
		},
		{
			name: "directory tree with ** is medium risk (read)",
			grantSet: &entities.GrantSet{
				FS: &entities.FileSystemCapability{
					Rules: []entities.FileSystemRule{{Read: []string{"/var/log/**"}}},
				},
			},
			expectLevel: entities.RiskMedium,
		},
		{
			name: "/etc/** is medium risk (read - sensitive system config)",
			grantSet: &entities.GrantSet{
				FS: &entities.FileSystemCapability{
					Rules: []entities.FileSystemRule{{Read: []string{"/etc/**"}}},
				},
			},
			expectLevel: entities.RiskMedium,
		},
		{
			name: "/home/** is medium risk (read - all user data)",
			grantSet: &entities.GrantSet{
				FS: &entities.FileSystemCapability{
					Rules: []entities.FileSystemRule{{Read: []string{"/home/**"}}},
				},
			},
			expectLevel: entities.RiskMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := assessor.Analyze(tt.grantSet)
			assert.Equal(t, tt.expectLevel, result.Level,
				"GrantSet should have expected risk level")
		})
	}
}

// TestRiskAssessor_EnvironmentWildcards verifies env wildcard detection.
// Note: SDK's SimpleRiskAnalyzer marks ALL env access as RiskLow.
func TestRiskAssessor_EnvironmentWildcards(t *testing.T) {
	assessor := entities.NewSimpleRiskAnalyzer()

	tests := []struct {
		name        string
		grantSet    *entities.GrantSet
		expectLevel entities.RiskLevel
	}{
		{
			name: "env:* is low risk (SDK doesn't distinguish)",
			grantSet: &entities.GrantSet{
				Env: &entities.EnvironmentCapability{Variables: []string{"*"}},
			},
			expectLevel: entities.RiskLow,
		},
		{
			name: "AWS_* is low risk (SDK doesn't distinguish)",
			grantSet: &entities.GrantSet{
				Env: &entities.EnvironmentCapability{Variables: []string{"AWS_*"}},
			},
			expectLevel: entities.RiskLow,
		},
		{
			name: "specific variable is low risk",
			grantSet: &entities.GrantSet{
				Env: &entities.EnvironmentCapability{Variables: []string{"AWS_REGION"}},
			},
			expectLevel: entities.RiskLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := assessor.Analyze(tt.grantSet)
			assert.Equal(t, tt.expectLevel, result.Level,
				"GrantSet should have expected risk level")
		})
	}
}

// TestRiskAssessor_NetworkWildcards verifies network wildcard detection.
// Note: SDK marks wildcard hosts as RiskCritical, specific hosts as RiskMedium.
func TestRiskAssessor_NetworkWildcards(t *testing.T) {
	assessor := entities.NewSimpleRiskAnalyzer()

	tests := []struct {
		name        string
		grantSet    *entities.GrantSet
		expectLevel entities.RiskLevel
	}{
		{
			name: "network:* host is critical risk",
			grantSet: &entities.GrantSet{
				Network: &entities.NetworkCapability{
					Rules: []entities.NetworkRule{{Hosts: []string{"*"}, Ports: []string{"443"}}},
				},
			},
			expectLevel: entities.RiskCritical,
		},
		{
			name: "specific host is medium risk",
			grantSet: &entities.GrantSet{
				Network: &entities.NetworkCapability{
					Rules: []entities.NetworkRule{{Hosts: []string{"api.example.com"}, Ports: []string{"443"}}},
				},
			},
			expectLevel: entities.RiskMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := assessor.Analyze(tt.grantSet)
			assert.Equal(t, tt.expectLevel, result.Level,
				"GrantSet should have expected risk level")
		})
	}
}

// TestRiskAssessor_VersionedInterpreters verifies that interpreters are detected as critical risk.
// Note: SDK's SimpleRiskAnalyzer marks ALL exec as critical - versioned or not.
func TestRiskAssessor_VersionedInterpreters(t *testing.T) {
	assessor := entities.NewSimpleRiskAnalyzer()

	tests := []struct {
		name     string
		grantSet *entities.GrantSet
	}{
		// Python versions - all should be detected as critical risk
		{
			name: "python (base) is critical risk",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"python"}},
			},
		},
		{
			name: "python3 is critical risk",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"python3"}},
			},
		},
		{
			name: "python3.11 is critical risk (versioned)",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"python3.11"}},
			},
		},
		// Node.js versions
		{
			name: "node is critical risk",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"node"}},
			},
		},
		{
			name: "node18 is critical risk (versioned)",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"node18"}},
			},
		},
		// Ruby versions
		{
			name: "ruby is critical risk",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"ruby"}},
			},
		},
		// AWK variants
		{
			name: "awk is critical risk",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"awk"}},
			},
		},
		{
			name: "gawk is critical risk",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"gawk"}},
			},
		},
		// Note: SDK doesn't distinguish safe vs dangerous commands
		{
			name: "specific command is also critical (SDK doesn't distinguish)",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"systemctl"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := assessor.Analyze(tt.grantSet)
			assert.Equal(t, entities.RiskCritical, result.Level,
				"All exec should be critical risk in SDK's SimpleRiskAnalyzer")
		})
	}
}
