package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/reglet-dev/reglet/internal/domain/capabilities"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/execution"
	"github.com/reglet-dev/reglet/internal/domain/values"
	"github.com/reglet-dev/reglet/internal/infrastructure/build"
	"github.com/reglet-dev/reglet/internal/infrastructure/wasm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testCapabilityManager is a simple mock that auto-approves all capabilities for testing
type testCapabilityManager struct {
	trustAll bool
}

func (m *testCapabilityManager) CollectRequiredCapabilities(ctx context.Context, profile entities.ProfileReader, runtime *wasm.Runtime, pluginDir string) (map[string][]capabilities.Capability, error) {
	// For tests, grant file plugin root filesystem access
	return map[string][]capabilities.Capability{
		"file": {
			{Kind: "fs", Pattern: "read:/**"},
		},
	}, nil
}

func (m *testCapabilityManager) GrantCapabilities(required map[string][]capabilities.Capability) (map[string][]capabilities.Capability, error) {
	if m.trustAll {
		return required, nil
	}
	return make(map[string][]capabilities.Capability), nil
}

// TestFiltering_EndToEnd simulates a full run with 20 controls and filtering.
func TestFiltering_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Locate plugins directory
	cwd, err := os.Getwd()
	require.NoError(t, err)

	// Walk up to find project root containing go.mod
	projectRoot := cwd
	for {
		if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(projectRoot)
		if parent == projectRoot {
			t.Fatal("could not find project root")
		}
		projectRoot = parent
	}
	pluginDir := filepath.Join(projectRoot, "plugins")

	// Create a temporary file that definitely exists and is accessible
	tempDir := t.TempDir()
	targetFile := filepath.Join(tempDir, "target.txt")
	err = os.WriteFile(targetFile, []byte("content"), 0o644)
	require.NoError(t, err)

	// 1. Define a profile with 20 controls
	// 5 controls: tag "target", severity "high"
	// 15 controls: tag "other", severity "low"
	var controls []entities.Control
	for i := 0; i < 20; i++ {
		tag := "other"
		severity := "low"
		if i < 5 {
			tag = "target"
			severity = "high"
		}

		ctrl := entities.Control{
			ID:       fmt.Sprintf("control-%d", i),
			Name:     fmt.Sprintf("Control %d", i),
			Severity: severity,
			Tags:     []string{tag},
			ObservationDefinitions: []entities.ObservationDefinition{
				{
					Plugin: "file",
					Config: map[string]interface{}{
						"path": "/", // Root should always exist if FS is mounted correctly
						"mode": "exists",
					},
				},
			},
		}
		controls = append(controls, ctrl)
	}

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "filtering-e2e-profile",
			Version: "1.0.0",
		},
		Controls: entities.ControlsSection{
			Items: controls,
		},
	}

	// 2. Configure Engine with filters (simulate --tags target)
	cfg := DefaultExecutionConfig()
	cfg.IncludeTags = []string{"target"}
	// Use parallel execution to stress test, but we handle result ordering
	cfg.Parallel = true

	// Create capability manager that trusts all plugins (auto-grant)
	capMgr := &testCapabilityManager{trustAll: true}

	// Initialize Engine with Capabilities and Config
	engine, err := NewEngineWithCapabilities(ctx, build.Get(), capMgr, pluginDir, profile, cfg, nil, nil, 0, &execution.GreedyTruncator{})
	require.NoError(t, err)
	defer engine.Close(ctx)

	// 3. Execute
	result, err := engine.Execute(ctx, profile)
	require.NoError(t, err)

	// 4. Verify Results

	// Total controls in profile is 20
	assert.Equal(t, 20, len(result.Controls))
	assert.Equal(t, 20, result.Summary.TotalControls)

	// Expected: 5 Passed (executed), 15 Skipped
	assert.Equal(t, 5, result.Summary.PassedControls, "Should have 5 passed controls")
	assert.Equal(t, 15, result.Summary.SkippedControls, "Should have 15 skipped controls")
	assert.Equal(t, 0, result.Summary.FailedControls)
	assert.Equal(t, 0, result.Summary.ErrorControls)

	// Map results by ID for verification (since parallel exec makes order non-deterministic)
	resultsMap := make(map[string]execution.ControlResult)
	for _, ctrl := range result.Controls {
		resultsMap[ctrl.ID] = ctrl
	}

	// Check individual controls
	for i := 0; i < 20; i++ {
		id := fmt.Sprintf("control-%d", i)
		ctrl, exists := resultsMap[id]
		require.True(t, exists, "Control %s missing from results", id)

		if i < 5 {
			// Should be executed and pass
			if !assert.Equal(t, values.StatusPass, ctrl.Status, "Control %s should pass", id) {
				// Dump details if failed
				t.Logf("Control %s failed. Message: %s", id, ctrl.Message)
				for _, obs := range ctrl.ObservationResults {
					t.Logf("  Observation status: %s", obs.Status)
					if obs.Error != nil {
						t.Logf("  Error: %v", obs.Error)
					}
					if obs.Evidence != nil {
						t.Logf("  Evidence: %v", obs.Evidence.Data)
					}
				}
			}
			assert.Empty(t, ctrl.SkipReason)
			assert.NotEmpty(t, ctrl.ObservationResults)
		} else {
			// Should be skipped
			assert.Equal(t, values.StatusSkipped, ctrl.Status, "Control %s should be skipped", id)
			assert.Contains(t, ctrl.SkipReason, "excluded by --tags filter", "Control %s skip reason incorrect", id)
			assert.Empty(t, ctrl.ObservationResults) // No observations should run
		}
	}
}
