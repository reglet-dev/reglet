package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	domainServices "github.com/whiskeyjimbo/reglet/internal/domain/services"
)

// TestCapabilityOrchestrator_UsesAnalyzer verifies that the orchestrator
// delegates capability extraction to the domain service.
func TestCapabilityOrchestrator_UsesAnalyzer(t *testing.T) {
	orchestrator := NewCapabilityOrchestrator(false)

	// Verify analyzer is injected
	require.NotNil(t, orchestrator.analyzer)
	assert.IsType(t, &domainServices.CapabilityAnalyzer{}, orchestrator.analyzer)
}

// TestCapabilityOrchestrator_SecurityLevels verifies security level configuration.
func TestCapabilityOrchestrator_SecurityLevels(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected string
	}{
		{
			name:     "Default security level",
			level:    "standard",
			expected: "standard",
		},
		{
			name:     "Strict security level",
			level:    "strict",
			expected: "strict",
		},
		{
			name:     "Permissive security level",
			level:    "permissive",
			expected: "permissive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orchestrator := NewCapabilityOrchestratorWithSecurity(false, tt.level)
			assert.Equal(t, tt.expected, orchestrator.securityLevel)
		})
	}
}

// Note: Comprehensive extraction logic tests are now in
// internal/domain/services/capability_analyzer_test.go
//
// This test file focuses on orchestration-specific behavior:
// - Delegation to domain services
// - Coordination of plugin loading and granting
// - Security policy application
