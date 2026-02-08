package services

import (
	"context"
	"testing"

	abi "github.com/reglet-dev/reglet-abi"
	"github.com/reglet-dev/reglet-abi/hostfunc"
	hostSDK "github.com/reglet-dev/reglet-host-sdk/capability"
	"github.com/reglet-dev/reglet-host-sdk/capability/gatekeeper"
	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	domainServices "github.com/reglet-dev/reglet/internal/domain/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPluginRuntimeFactory is a test double for PluginRuntimeFactory.
type mockPluginRuntimeFactory struct{}

func (m *mockPluginRuntimeFactory) NewRuntime(_ context.Context, _ ...ports.RuntimeOption) (ports.PluginRuntime, error) {
	return &mockPluginRuntime{}, nil
}

// mockPluginRuntime is a test double for PluginRuntime.
type mockPluginRuntime struct{}

func (m *mockPluginRuntime) LoadPlugin(_ context.Context, _ string, _ []byte) (ports.Plugin, error) {
	return &mockPlugin{}, nil
}

func (m *mockPluginRuntime) Close(_ context.Context) error {
	return nil
}

// mockPlugin is a test double for Plugin.
type mockPlugin struct{}

func (m *mockPlugin) Manifest(_ context.Context) (*abi.Manifest, error) {
	return &abi.Manifest{
		Name:         "mock",
		Version:      "1.0.0",
		Capabilities: hostfunc.GrantSet{}, // Empty GrantSet (ABI)
	}, nil
}

func (m *mockPlugin) RequiredCapabilities(_ context.Context) (*hostfunc.GrantSet, error) {
	return &hostfunc.GrantSet{}, nil
}

// TestCapabilityOrchestrator_UsesAnalyzer verifies that the orchestrator
// delegates capability extraction to the domain service.
func TestCapabilityOrchestrator_UsesAnalyzer(t *testing.T) {
	orchestrator := NewCapabilityOrchestrator(
		&mockPluginRuntimeFactory{},
		WithCapabilityRegistry(hostSDK.NewRegistry()),
	)

	// Verify analyzer is injected
	require.NotNil(t, orchestrator.analyzer)
	assert.IsType(t, &domainServices.CapabilityAnalyzer{}, orchestrator.analyzer)
}

// TestCapabilityOrchestrator_UsesGatekeeper verifies that the orchestrator
// delegates granting to the gatekeeper.
func TestCapabilityOrchestrator_UsesGatekeeper(t *testing.T) {
	orchestrator := NewCapabilityOrchestrator(
		&mockPluginRuntimeFactory{},
		WithCapabilityRegistry(hostSDK.NewRegistry()),
	)

	// Verify gatekeeper is injected
	require.NotNil(t, orchestrator.gatekeeper)
	assert.IsType(t, &gatekeeper.Gatekeeper{}, orchestrator.gatekeeper)
}

// mockCapabilityGatekeeper is a test double for the gatekeeper interface.
type mockCapabilityGatekeeper struct {
	grantCalled bool
	trustAll    bool
	grantResult *hostfunc.GrantSet
	grantError  error
	hasResult   bool
}

func (m *mockCapabilityGatekeeper) GrantCapabilities(
	required *hostfunc.GrantSet,
	_ map[string]ports.CapabilityInfo,
	trustAll bool,
) (*hostfunc.GrantSet, error) {
	m.grantCalled = true
	m.trustAll = trustAll
	if m.hasResult {
		return m.grantResult, m.grantError
	}
	return required, m.grantError
}

// TestCapabilityOrchestrator_WithMockGatekeeper verifies the orchestrator
// correctly delegates to the injected gatekeeper.
func TestCapabilityOrchestrator_WithMockGatekeeper(t *testing.T) {
	// Create mock analyzer (domain service implements the interface)
	analyzer := domainServices.NewCapabilityAnalyzer(hostSDK.NewRegistry())

	// Create mock gatekeeper
	mockGK := &mockCapabilityGatekeeper{
		hasResult: true,
		grantResult: &hostfunc.GrantSet{
			FS: &hostfunc.FileSystemCapability{
				Rules: []hostfunc.FileSystemRule{
					{Read: []string{"/etc/passwd"}},
				},
			},
		},
	}

	// Create orchestrator with injected dependencies
	orchestrator := NewCapabilityOrchestrator(
		&mockPluginRuntimeFactory{},
		WithAnalyzer(analyzer),
		WithGatekeeper(mockGK),
	)

	// Test GrantCapabilities delegates to the mock
	required := map[string]*hostfunc.GrantSet{
		"file": {
			FS: &hostfunc.FileSystemCapability{
				Rules: []hostfunc.FileSystemRule{
					{Read: []string{"/etc/passwd"}},
				},
			},
		},
	}
	granted, err := orchestrator.GrantCapabilities(required, false)

	require.NoError(t, err)
	assert.True(t, mockGK.grantCalled, "gatekeeper should have been called")
	assert.NotNil(t, granted)
}

// TestCapabilityOrchestrator_ErrorPropagation verifies that errors from the
// gatekeeper are correctly propagated to the caller.
func TestCapabilityOrchestrator_ErrorPropagation(t *testing.T) {
	analyzer := domainServices.NewCapabilityAnalyzer(hostSDK.NewRegistry())

	// Create mock gatekeeper that returns an error
	mockGK := &mockCapabilityGatekeeper{
		hasResult:  true,
		grantError: assert.AnError, // Use testify's standard error
	}

	orchestrator := NewCapabilityOrchestrator(
		&mockPluginRuntimeFactory{},
		WithAnalyzer(analyzer),
		WithGatekeeper(mockGK),
	)

	required := map[string]*hostfunc.GrantSet{
		"file": {
			FS: &hostfunc.FileSystemCapability{
				Rules: []hostfunc.FileSystemRule{
					{Read: []string{"/etc/passwd"}},
				},
			},
		},
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
			analyzer := domainServices.NewCapabilityAnalyzer(hostSDK.NewRegistry())
			mockGK := &mockCapabilityGatekeeper{
				hasResult:   true,
				grantResult: &hostfunc.GrantSet{},
			}

			orchestrator := NewCapabilityOrchestrator(
				&mockPluginRuntimeFactory{},
				WithAnalyzer(analyzer),
				WithGatekeeper(mockGK),
				WithTrustAll(tt.orchestratorTrust),
			)

			required := map[string]*hostfunc.GrantSet{
				"file": {
					FS: &hostfunc.FileSystemCapability{
						Rules: []hostfunc.FileSystemRule{
							{Read: []string{"/etc/passwd"}},
						},
					},
				},
			}
			_, err := orchestrator.GrantCapabilities(required, tt.grantTrust)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedTrust, mockGK.trustAll, "trustAll should be correctly propagated")
		})
	}
}

// mockCapabilityAnalyzer is a test double for the analyzer interface.
type mockCapabilityAnalyzer struct {
	extractedCaps map[string]*hostfunc.GrantSet
	extractCalled bool
}

func (m *mockCapabilityAnalyzer) ExtractCapabilities(_ entities.ProfileReader) map[string]*hostfunc.GrantSet {
	m.extractCalled = true
	return m.extractedCaps
}

// TestCapabilityOrchestrator_WithMockAnalyzer verifies the orchestrator
// correctly uses the injected analyzer.
func TestCapabilityOrchestrator_WithMockAnalyzer(t *testing.T) {
	// Create mock analyzer with predefined capabilities
	mockAnalyzer := &mockCapabilityAnalyzer{
		extractedCaps: map[string]*hostfunc.GrantSet{
			"file": {
				FS: &hostfunc.FileSystemCapability{
					Rules: []hostfunc.FileSystemRule{
						{Read: []string{"/etc/passwd"}},
					},
				},
			},
		},
	}

	mockGK := &mockCapabilityGatekeeper{
		hasResult: true,
		grantResult: &hostfunc.GrantSet{
			FS: &hostfunc.FileSystemCapability{
				Rules: []hostfunc.FileSystemRule{
					{Read: []string{"/etc/passwd"}},
				},
			},
		},
	}

	orchestrator := NewCapabilityOrchestrator(
		&mockPluginRuntimeFactory{},
		WithAnalyzer(mockAnalyzer),
		WithGatekeeper(mockGK),
	)

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
