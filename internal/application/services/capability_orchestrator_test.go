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

// TestCapabilityOrchestrator_UsesGatekeeper verifies that the orchestrator
// delegates granting to the gatekeeper.
func TestCapabilityOrchestrator_UsesGatekeeper(t *testing.T) {
	orchestrator := NewCapabilityOrchestrator(false)

	// Verify gatekeeper is injected
	require.NotNil(t, orchestrator.gatekeeper)
	assert.IsType(t, &CapabilityGatekeeper{}, orchestrator.gatekeeper)
}

// Note: Comprehensive extraction logic tests are now in
// internal/domain/services/capability_analyzer_test.go
//
// This test file focuses on orchestration-specific behavior:
// - Delegation to domain services
// - Coordination of plugin loading and granting
// - Security policy application
