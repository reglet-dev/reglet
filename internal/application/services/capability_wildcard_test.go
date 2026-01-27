package services

import (
	"testing"

	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/stretchr/testify/assert"
)

// TestRiskAssessor_ExecWildcards verifies that exec wildcard patterns are detected as high risk.
// This prevents undermining the principle of least privilege.
func TestRiskAssessor_ExecWildcards(t *testing.T) {
	assessor := entities.NewRiskAssessor()

	tests := []struct {
		name     string
		grantSet *entities.GrantSet
		isHigh   bool
	}{
		{
			name: "exec:** is high risk (arbitrary command execution)",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"**"}},
			},
			isHigh: true,
		},
		{
			name: "exec:* is high risk",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"*"}},
			},
			isHigh: true,
		},
		{
			name: "specific binary is not high risk",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"/usr/bin/ls"}},
			},
			isHigh: false,
		},
		{
			name: "shells are high risk (allows arbitrary commands)",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"/bin/sh"}},
			},
			isHigh: true,
		},
		{
			name: "interpreters are high risk (allows code execution via -c)",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"python"}},
			},
			isHigh: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := assessor.AssessGrantSet(tt.grantSet)
			if tt.isHigh {
				assert.Equal(t, entities.RiskLevelHigh, result,
					"GrantSet should be detected as high risk")
			} else {
				assert.NotEqual(t, entities.RiskLevelHigh, result,
					"GrantSet should NOT be detected as high risk")
			}
		})
	}
}

// TestRiskAssessor_FilesystemWildcards verifies that fs wildcard patterns are detected.
func TestRiskAssessor_FilesystemWildcards(t *testing.T) {
	assessor := entities.NewRiskAssessor()

	tests := []struct {
		name     string
		grantSet *entities.GrantSet
		isHigh   bool
	}{
		{
			name: "fs:read:** is high risk",
			grantSet: &entities.GrantSet{
				FS: &entities.FileSystemCapability{
					Rules: []entities.FileSystemRule{{Read: []string{"**"}}},
				},
			},
			isHigh: true,
		},
		{
			name: "fs:write:** is high risk",
			grantSet: &entities.GrantSet{
				FS: &entities.FileSystemCapability{
					Rules: []entities.FileSystemRule{{Write: []string{"**"}}},
				},
			},
			isHigh: true,
		},
		{
			name: "root filesystem is high risk",
			grantSet: &entities.GrantSet{
				FS: &entities.FileSystemCapability{
					Rules: []entities.FileSystemRule{{Read: []string{"/**"}}},
				},
			},
			isHigh: true,
		},
		{
			name: "specific file is not high risk",
			grantSet: &entities.GrantSet{
				FS: &entities.FileSystemCapability{
					Rules: []entities.FileSystemRule{{Read: []string{"/etc/passwd"}}},
				},
			},
			isHigh: false,
		},
		{
			name: "directory tree with ** is high risk",
			grantSet: &entities.GrantSet{
				FS: &entities.FileSystemCapability{
					Rules: []entities.FileSystemRule{{Read: []string{"/var/log/**"}}},
				},
			},
			isHigh: true, // SDK considers any ** pattern as high risk
		},
		{
			name: "/etc/** is high risk (sensitive system config)",
			grantSet: &entities.GrantSet{
				FS: &entities.FileSystemCapability{
					Rules: []entities.FileSystemRule{{Read: []string{"/etc/**"}}},
				},
			},
			isHigh: true,
		},
		{
			name: "/home/** is high risk (all user data)",
			grantSet: &entities.GrantSet{
				FS: &entities.FileSystemCapability{
					Rules: []entities.FileSystemRule{{Read: []string{"/home/**"}}},
				},
			},
			isHigh: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := assessor.AssessGrantSet(tt.grantSet)
			if tt.isHigh {
				assert.Equal(t, entities.RiskLevelHigh, result,
					"GrantSet should be detected as high risk")
			} else {
				assert.NotEqual(t, entities.RiskLevelHigh, result,
					"GrantSet should NOT be detected as high risk")
			}
		})
	}
}

// TestRiskAssessor_EnvironmentWildcards verifies env wildcard detection.
func TestRiskAssessor_EnvironmentWildcards(t *testing.T) {
	assessor := entities.NewRiskAssessor()

	tests := []struct {
		name     string
		grantSet *entities.GrantSet
		isHigh   bool
	}{
		{
			name: "env:* is high risk (all environment variables)",
			grantSet: &entities.GrantSet{
				Env: &entities.EnvironmentCapability{Variables: []string{"*"}},
			},
			isHigh: true,
		},
		{
			name: "AWS_* is high risk (all AWS credentials)",
			grantSet: &entities.GrantSet{
				Env: &entities.EnvironmentCapability{Variables: []string{"AWS_*"}},
			},
			isHigh: true,
		},
		{
			name: "specific variable is not high risk",
			grantSet: &entities.GrantSet{
				Env: &entities.EnvironmentCapability{Variables: []string{"AWS_REGION"}},
			},
			isHigh: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := assessor.AssessGrantSet(tt.grantSet)
			if tt.isHigh {
				assert.Equal(t, entities.RiskLevelHigh, result,
					"GrantSet should be detected as high risk")
			} else {
				assert.NotEqual(t, entities.RiskLevelHigh, result,
					"GrantSet should NOT be detected as high risk")
			}
		})
	}
}

// TestRiskAssessor_NetworkWildcards verifies network wildcard detection.
func TestRiskAssessor_NetworkWildcards(t *testing.T) {
	assessor := entities.NewRiskAssessor()

	tests := []struct {
		name     string
		grantSet *entities.GrantSet
		isHigh   bool
	}{
		{
			name: "network:* host is high risk",
			grantSet: &entities.GrantSet{
				Network: &entities.NetworkCapability{
					Rules: []entities.NetworkRule{{Hosts: []string{"*"}, Ports: []string{"443"}}},
				},
			},
			isHigh: true,
		},
		{
			name: "specific host is not high risk",
			grantSet: &entities.GrantSet{
				Network: &entities.NetworkCapability{
					Rules: []entities.NetworkRule{{Hosts: []string{"api.example.com"}, Ports: []string{"443"}}},
				},
			},
			isHigh: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := assessor.AssessGrantSet(tt.grantSet)
			if tt.isHigh {
				assert.Equal(t, entities.RiskLevelHigh, result,
					"GrantSet should be detected as high risk")
			} else {
				assert.NotEqual(t, entities.RiskLevelHigh, result,
					"GrantSet should NOT be detected as high risk")
			}
		})
	}
}

// TestRiskAssessor_VersionedInterpreters verifies that versioned interpreters are detected as high risk.
// This prevents bypass attacks using python3.11 instead of python3, node18 instead of node, etc.
func TestRiskAssessor_VersionedInterpreters(t *testing.T) {
	assessor := entities.NewRiskAssessor()

	tests := []struct {
		name     string
		grantSet *entities.GrantSet
		isHigh   bool
	}{
		// Python versions - all should be detected as high risk
		{
			name: "python (base) is high risk",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"python"}},
			},
			isHigh: true,
		},
		{
			name: "python3 is high risk",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"python3"}},
			},
			isHigh: true,
		},
		{
			name: "python3.11 is high risk (versioned)",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"python3.11"}},
			},
			isHigh: true,
		},
		// Node.js versions
		{
			name: "node is high risk",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"node"}},
			},
			isHigh: true,
		},
		{
			name: "node18 is high risk (versioned)",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"node18"}},
			},
			isHigh: true,
		},
		// Ruby versions
		{
			name: "ruby is high risk",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"ruby"}},
			},
			isHigh: true,
		},
		// AWK variants
		{
			name: "awk is high risk",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"awk"}},
			},
			isHigh: true,
		},
		{
			name: "gawk is high risk",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"gawk"}},
			},
			isHigh: true,
		},
		// Negative tests - should NOT be detected as high risk
		{
			name: "specific command is NOT high risk",
			grantSet: &entities.GrantSet{
				Exec: &entities.ExecCapability{Commands: []string{"systemctl"}},
			},
			isHigh: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := assessor.AssessGrantSet(tt.grantSet)
			if tt.isHigh {
				assert.Equal(t, entities.RiskLevelHigh, result,
					"GrantSet should be detected as high risk")
			} else {
				assert.NotEqual(t, entities.RiskLevelHigh, result,
					"GrantSet should NOT be detected as high risk")
			}
		})
	}
}
