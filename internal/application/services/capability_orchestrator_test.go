package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/whiskeyjimbo/reglet/internal/application/ports"
	"github.com/whiskeyjimbo/reglet/internal/domain/capabilities"
	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
	domainServices "github.com/whiskeyjimbo/reglet/internal/domain/services"
)

// TestCapabilityOrchestrator_UsesAnalyzer verifies that the orchestrator
// delegates capability extraction to the domain service.
func TestCapabilityOrchestrator_UsesAnalyzer(t *testing.T) {
	orchestrator := NewCapabilityOrchestrator("", false, capabilities.NewRegistry())

	// Verify analyzer is injected
	require.NotNil(t, orchestrator.analyzer)
	assert.IsType(t, &domainServices.CapabilityAnalyzer{}, orchestrator.analyzer)
}

// TestCapabilityOrchestrator_UsesGatekeeper verifies that the orchestrator
// delegates granting to the gatekeeper.
func TestCapabilityOrchestrator_UsesGatekeeper(t *testing.T) {
	orchestrator := NewCapabilityOrchestrator("", false, capabilities.NewRegistry())

	// Verify gatekeeper is injected
	require.NotNil(t, orchestrator.gatekeeper)
	assert.IsType(t, &CapabilityGatekeeper{}, orchestrator.gatekeeper)
}

// mockCapabilityGatekeeper is a test double for the gatekeeper interface.
type mockCapabilityGatekeeper struct {
	grantCalled bool
	trustAll    bool
	grantResult capabilities.Grant
	grantError  error
}

func (m *mockCapabilityGatekeeper) GrantCapabilities(
	required capabilities.Grant,
	_ map[string]ports.CapabilityInfo,
	trustAll bool,
) (capabilities.Grant, error) {
	m.grantCalled = true
	m.trustAll = trustAll
	if m.grantResult != nil {
		return m.grantResult, m.grantError
	}
	return required, m.grantError
}

// TestCapabilityOrchestrator_WithMockGatekeeper verifies the orchestrator
// correctly delegates to the injected gatekeeper.
func TestCapabilityOrchestrator_WithMockGatekeeper(t *testing.T) {
	// Create mock analyzer (domain service implements the interface)
	analyzer := domainServices.NewCapabilityAnalyzer(capabilities.NewRegistry())

	// Create mock gatekeeper
	mockGK := &mockCapabilityGatekeeper{
		grantResult: capabilities.NewGrant(),
	}
	mockGK.grantResult.Add(capabilities.Capability{Kind: "fs", Pattern: "read:/etc/passwd"})

	// Create orchestrator with injected dependencies
	orchestrator := NewCapabilityOrchestratorWithDeps(analyzer, mockGK, false)

	// Test GrantCapabilities delegates to the mock
	required := map[string][]capabilities.Capability{
		"file": {{Kind: "fs", Pattern: "read:/etc/passwd"}},
	}
	granted, err := orchestrator.GrantCapabilities(required, false)

	require.NoError(t, err)
	assert.True(t, mockGK.grantCalled, "gatekeeper should have been called")
	assert.NotEmpty(t, granted)
}

// TestCapabilityOrchestrator_ErrorPropagation verifies that errors from the
// gatekeeper are correctly propagated to the caller.
func TestCapabilityOrchestrator_ErrorPropagation(t *testing.T) {
	analyzer := domainServices.NewCapabilityAnalyzer(capabilities.NewRegistry())

	// Create mock gatekeeper that returns an error
	mockGK := &mockCapabilityGatekeeper{
		grantError: assert.AnError, // Use testify's standard error
	}

	orchestrator := NewCapabilityOrchestratorWithDeps(analyzer, mockGK, false)

	required := map[string][]capabilities.Capability{
		"file": {{Kind: "fs", Pattern: "read:/etc/passwd"}},
	}
	_, err := orchestrator.GrantCapabilities(required, false)

	assert.Error(t, err, "error should propagate from gatekeeper")
	assert.True(t, mockGK.grantCalled, "gatekeeper should have been called")
}

// TestCapabilityOrchestrator_TrustAllFlagPropagation verifies that the trustAll
// flag is correctly passed through to the gatekeeper.
func TestCapabilityOrchestrator_TrustAllFlagPropagation(t *testing.T) {
	tests := []struct {
		name              string
		orchestratorTrust bool // trustAll set on orchestrator constructor
		grantTrust        bool // trustAll passed to GrantCapabilities
		expectedTrust     bool // what the gatekeeper should receive
	}{
		{
			name:              "both false results in false",
			orchestratorTrust: false,
			grantTrust:        false,
			expectedTrust:     false,
		},
		{
			name:              "orchestrator trust overrides",
			orchestratorTrust: true,
			grantTrust:        false,
			expectedTrust:     true,
		},
		{
			name:              "grant trust overrides",
			orchestratorTrust: false,
			grantTrust:        true,
			expectedTrust:     true,
		},
		{
			name:              "both true results in true",
			orchestratorTrust: true,
			grantTrust:        true,
			expectedTrust:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := domainServices.NewCapabilityAnalyzer(capabilities.NewRegistry())
			mockGK := &mockCapabilityGatekeeper{
				grantResult: capabilities.NewGrant(),
			}

			orchestrator := NewCapabilityOrchestratorWithDeps(analyzer, mockGK, tt.orchestratorTrust)

			required := map[string][]capabilities.Capability{
				"file": {{Kind: "fs", Pattern: "read:/etc/passwd"}},
			}
			_, err := orchestrator.GrantCapabilities(required, tt.grantTrust)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedTrust, mockGK.trustAll, "trustAll should be correctly propagated")
		})
	}
}

// mockCapabilityAnalyzer is a test double for the analyzer interface.
type mockCapabilityAnalyzer struct {
	extractedCaps map[string][]capabilities.Capability
	extractCalled bool
}

func (m *mockCapabilityAnalyzer) ExtractCapabilities(_ entities.ProfileReader) map[string][]capabilities.Capability {
	m.extractCalled = true
	return m.extractedCaps
}

// TestCapabilityOrchestrator_WithMockAnalyzer verifies the orchestrator
// correctly uses the injected analyzer.
func TestCapabilityOrchestrator_WithMockAnalyzer(t *testing.T) {
	// Create mock analyzer with predefined capabilities
	mockAnalyzer := &mockCapabilityAnalyzer{
		extractedCaps: map[string][]capabilities.Capability{
			"file": {{Kind: "fs", Pattern: "read:/etc/passwd"}},
		},
	}

	mockGK := &mockCapabilityGatekeeper{
		grantResult: capabilities.NewGrant(),
	}
	mockGK.grantResult.Add(capabilities.Capability{Kind: "fs", Pattern: "read:/etc/passwd"})

	orchestrator := NewCapabilityOrchestratorWithDeps(mockAnalyzer, mockGK, false)

	// GrantCapabilities doesn't call the analyzer directly (it's called in CollectRequiredCapabilities)
	// but we can verify the orchestrator was constructed with the mock
	require.NotNil(t, orchestrator.analyzer)
	assert.Equal(t, mockAnalyzer, orchestrator.analyzer)
}

// Note: Comprehensive extraction logic tests are now in
// internal/domain/services/capability_analyzer_test.go
//
// This test file focuses on orchestration-specific behavior:
// - Delegation to domain services
// - Coordination of plugin loading and granting
// - Security policy application
