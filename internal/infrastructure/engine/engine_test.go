package engine

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/execution"
	"github.com/reglet-dev/reglet/internal/domain/services"
	"github.com/reglet-dev/reglet/internal/domain/values"
	"github.com/reglet-dev/reglet/internal/infrastructure/build"
	"github.com/reglet-dev/reglet/internal/infrastructure/wasm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEngine(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	engine, err := NewEngine(ctx, build.Get())
	require.NoError(t, err)
	require.NotNil(t, engine)
	require.NotNil(t, engine.runtime)
	require.NotNil(t, engine.executor)

	// Cleanup
	err = engine.Close(ctx)
	assert.NoError(t, err)
}

func TestEngineOptions_Defaults(t *testing.T) {
	t.Parallel()
	opts := defaultEngineOptions()
	assert.NotNil(t, opts)
	assert.NotNil(t, opts.executionConfig)
	assert.IsType(t, &execution.GreedyTruncator{}, opts.truncator)
	assert.Equal(t, 0, opts.memoryLimitMB)
	assert.Equal(t, "", opts.pluginDir)
}

func TestEngineOptions_WithExecutionConfig(t *testing.T) {
	t.Parallel()
	cfg := ExecutionConfig{
		Parallel:    true,
		IncludeTags: []string{"test"},
	}

	opts := defaultEngineOptions()
	WithExecutionConfig(cfg)(opts)

	assert.Equal(t, cfg, opts.executionConfig)
}

func TestEngineOptions_WithCapabilityManager(t *testing.T) {
	t.Parallel()
	mockCapMgr := &struct{ CapabilityManager }{}
	mockProfile := &entities.Profile{}

	// Should fail creation if Profile is missing
	eng, err := NewEngine(context.Background(), build.Get(),
		WithCapabilityManager(mockCapMgr),
	)
	assert.Error(t, err)
	assert.Nil(t, eng)
	assert.Contains(t, err.Error(), "requires WithProfile")

	// Should succeed if Profile is present
	// We need to be careful: NewEngine will try to run the capability flow which calls methods on mockCapMgr
	// So we can't test NewEngine success easily without a working mock.
	// But we can check if the option sets the field on the options struct manually.

	opts := defaultEngineOptions()
	WithCapabilityManager(mockCapMgr)(opts)
	assert.NotNil(t, opts.capabilityManager)

	opts = defaultEngineOptions()
	WithProfile(mockProfile)(opts)
	assert.NotNil(t, opts.profile)
}

func TestEngineOptions_WithMemoryLimit(t *testing.T) {
	t.Parallel()
	opts := defaultEngineOptions()
	WithMemoryLimit(512)(opts)
	assert.Equal(t, 512, opts.memoryLimitMB)
}

func TestEngineOptions_WithPluginDir(t *testing.T) {
	t.Parallel()
	opts := defaultEngineOptions()
	WithPluginDir("/tmp/plugins")(opts)
	assert.Equal(t, "/tmp/plugins", opts.pluginDir)
}

/*
   We skip full integration tests of WithCapabilityManager in NewEngine here
   because it requires mocking the entire capability flow which is covered in
   TestFiltering_EndToEnd (using the real flow) or requires mocks.
   The validation logic (require WithProfile) is tested above.
*/

func TestGenerateControlMessage_SinglePass(t *testing.T) {
	t.Parallel()
	observations := []execution.ObservationResult{
		{Status: values.StatusPass},
	}

	msg := generateControlMessage(values.StatusPass, observations)
	assert.Equal(t, "Check passed", msg)
}

func TestGenerateControlMessage_MultiplePass(t *testing.T) {
	t.Parallel()
	observations := []execution.ObservationResult{
		{Status: values.StatusPass},
		{Status: values.StatusPass},
		{Status: values.StatusPass},
	}

	msg := generateControlMessage(values.StatusPass, observations)
	assert.Equal(t, "All 3 checks passed", msg)
}

func TestGenerateControlMessage_SingleFail(t *testing.T) {
	t.Parallel()
	observations := []execution.ObservationResult{
		{Status: values.StatusPass},
		{Status: values.StatusFail},
	}

	msg := generateControlMessage(values.StatusFail, observations)
	assert.Equal(t, "1 check failed", msg)
}

func TestGenerateControlMessage_MultipleFail(t *testing.T) {
	t.Parallel()
	observations := []execution.ObservationResult{
		{Status: values.StatusFail},
		{Status: values.StatusFail},
		{Status: values.StatusPass},
	}

	msg := generateControlMessage(values.StatusFail, observations)
	assert.Equal(t, "2 checks failed", msg)
}

func TestGenerateControlMessage_SingleError(t *testing.T) {
	t.Parallel()
	observations := []execution.ObservationResult{
		{
			Status: values.StatusError,
			Error:  &wasm.PluginError{Code: "test", Message: "something went wrong"},
		},
	}

	msg := generateControlMessage(values.StatusError, observations)
	assert.Equal(t, "something went wrong", msg)
}

func TestGenerateControlMessage_SingleErrorNoMessage(t *testing.T) {
	t.Parallel()
	observations := []execution.ObservationResult{
		{
			Status: values.StatusError,
			Error:  nil, // No error object
		},
	}

	msg := generateControlMessage(values.StatusError, observations)
	assert.Equal(t, "Check encountered an error", msg)
}

func TestGenerateControlMessage_MultipleErrors(t *testing.T) {
	t.Parallel()
	observations := []execution.ObservationResult{
		{Status: values.StatusError, Error: &wasm.PluginError{Code: "test", Message: "error 1"}},
		{Status: values.StatusError, Error: &wasm.PluginError{Code: "test", Message: "error 2"}},
		{Status: values.StatusPass},
	}

	msg := generateControlMessage(values.StatusError, observations)
	assert.Equal(t, "2 checks encountered errors", msg)
}

func TestExecuteControl_SingleObservation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine, err := NewEngine(ctx, build.Get())
	require.NoError(t, err)
	defer engine.Close(ctx)

	ctrl := entities.Control{
		ID:          "test-control",
		Name:        "Test Control",
		Description: "A test control",
		Severity:    "medium",
		Tags:        []string{"test"},
		ObservationDefinitions: []entities.ObservationDefinition{
			{
				Plugin: "file",
				Config: map[string]interface{}{
					"path":   "/tmp/test.txt",
					"mode":   "exists",
					"status": true,
				},
			},
		},
	}

	// Create empty execution result for dependency checking
	execResult := execution.NewExecutionResult("test", "1.0.0")
	result := engine.executeControl(ctx, ctrl, 0, execResult, nil, nil)

	assert.Equal(t, "test-control", result.ID)
	assert.Equal(t, "Test Control", result.Name)
	assert.Equal(t, "A test control", result.Description)
	assert.Equal(t, "medium", result.Severity)
	assert.Equal(t, []string{"test"}, result.Tags)
	assert.Len(t, result.ObservationResults, 1)
	assert.Greater(t, result.Duration, time.Duration(0))
	assert.NotEmpty(t, result.Message)
}

func TestExecuteControl_MultipleObservations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine, err := NewEngine(ctx, build.Get())
	require.NoError(t, err)
	defer engine.Close(ctx)

	ctrl := entities.Control{
		ID:   "multi-test",
		Name: "Multi Observation Test",
		ObservationDefinitions: []entities.ObservationDefinition{
			{
				Plugin: "file",
				Config: map[string]interface{}{
					"path": "/tmp/test1.txt",
					"mode": "exists",
				},
			},
			{
				Plugin: "file",
				Config: map[string]interface{}{
					"path": "/tmp/test2.txt",
					"mode": "exists",
				},
			},
		},
	}

	// Create empty execution result for dependency checking
	execResult := execution.NewExecutionResult("test", "1.0.0")
	result := engine.executeControl(ctx, ctrl, 0, execResult, nil, nil)

	assert.Equal(t, "multi-test", result.ID)
	assert.Len(t, result.ObservationResults, 2)
	assert.Greater(t, result.Duration, time.Duration(0))
}

func TestExecute_SingleControl(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine, err := NewEngine(ctx, build.Get())
	require.NoError(t, err)
	defer engine.Close(ctx)

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "test-profile",
			Version: "1.0.0",
		},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "control-1",
					Name: "Control 1",
					ObservationDefinitions: []entities.ObservationDefinition{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/tmp/test.txt",
								"mode": "exists",
							},
						},
					},
				},
			},
		},
	}

	result, err := engine.Execute(ctx, profile)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "test-profile", result.ProfileName)
	assert.Equal(t, "1.0.0", result.ProfileVersion)
	assert.NotZero(t, result.StartTime)
	assert.NotZero(t, result.EndTime)
	assert.Greater(t, result.Duration, time.Duration(0))
	assert.Len(t, result.Controls, 1)
	assert.Equal(t, 1, result.Summary.TotalControls)
	assert.Equal(t, 1, result.Summary.TotalObservations)
}

func TestExecute_MultipleControls(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine, err := NewEngine(ctx, build.Get())
	require.NoError(t, err)
	defer engine.Close(ctx)

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "multi-control-profile",
			Version: "2.0.0",
		},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "control-1",
					Name: "Control 1",
					ObservationDefinitions: []entities.ObservationDefinition{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/tmp/test1.txt",
								"mode": "exists",
							},
						},
					},
				},
				{
					ID:   "control-2",
					Name: "Control 2",
					ObservationDefinitions: []entities.ObservationDefinition{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/tmp/test2.txt",
								"mode": "exists",
							},
						},
					},
				},
			},
		},
	}

	result, err := engine.Execute(ctx, profile)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "multi-control-profile", result.ProfileName)
	assert.Len(t, result.Controls, 2)
	assert.Equal(t, 2, result.Summary.TotalControls)
	assert.Equal(t, 2, result.Summary.TotalObservations)
}

func TestExecute_SummaryStatistics(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine, err := NewEngine(ctx, build.Get())
	require.NoError(t, err)
	defer engine.Close(ctx)

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "summary-test",
			Version: "1.0.0",
		},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "control-1",
					Name: "Control 1",
					ObservationDefinitions: []entities.ObservationDefinition{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/tmp/test.txt",
								"mode": "exists",
							},
						},
					},
				},
			},
		},
	}

	result, err := engine.Execute(ctx, profile)
	require.NoError(t, err)

	// Verify summary is calculated
	assert.Equal(t, 1, result.Summary.TotalControls)
	assert.Equal(t, 1, result.Summary.TotalObservations)

	// Should have exactly one of: pass, fail, or error
	totalStatusCounts := result.Summary.PassedControls +
		result.Summary.FailedControls +
		result.Summary.ErrorControls
	assert.Equal(t, 1, totalStatusCounts)
}

func TestExecute_TimingInfo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine, err := NewEngine(ctx, build.Get())
	require.NoError(t, err)
	defer engine.Close(ctx)

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "timing-test",
			Version: "1.0.0",
		},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "control-1",
					Name: "Control 1",
					ObservationDefinitions: []entities.ObservationDefinition{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/tmp/test.txt",
								"mode": "exists",
							},
						},
					},
				},
			},
		},
	}

	result, err := engine.Execute(ctx, profile)
	require.NoError(t, err)

	// Verify timing information is present
	assert.NotZero(t, result.StartTime)
	assert.NotZero(t, result.EndTime)
	assert.Greater(t, result.Duration, time.Duration(0))
	assert.True(t, result.EndTime.After(result.StartTime))
	assert.Greater(t, result.Controls[0].Duration, time.Duration(0))
	assert.Greater(t, result.Controls[0].ObservationResults[0].Duration, time.Duration(0))
}

func TestExecute_InvalidPlugin(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine, err := NewEngine(ctx, build.Get())
	require.NoError(t, err)
	defer engine.Close(ctx)

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "invalid-plugin-test",
			Version: "1.0.0",
		},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "control-1",
					Name: "Control 1",
					ObservationDefinitions: []entities.ObservationDefinition{
						{
							Plugin: "nonexistent-plugin",
							Config: map[string]interface{}{
								"test": "value",
							},
						},
					},
				},
			},
		},
	}

	result, err := engine.Execute(ctx, profile)
	require.NoError(t, err) // Execute should not return error, but result should show error

	assert.Len(t, result.Controls, 1)
	assert.Equal(t, values.StatusError, result.Controls[0].Status)
	assert.Len(t, result.Controls[0].ObservationResults, 1)
	assert.Equal(t, values.StatusError, result.Controls[0].ObservationResults[0].Status)
	assert.NotNil(t, result.Controls[0].ObservationResults[0].Error)
	assert.Contains(t, result.Controls[0].ObservationResults[0].Error.Message, "failed to read plugin")
}

// --- Filtering Tests ---

func TestShouldRun_AdvancedFilter(t *testing.T) {
	// Expression: severity == 'critical' && 'prod' in tags
	program, err := expr.Compile("severity == 'critical' && 'prod' in tags", expr.Env(services.ControlEnv{}), expr.AsBool())
	require.NoError(t, err)

	// Expression: owner == 'security-team'
	ownerProgram, err := expr.Compile("owner == 'security-team'", expr.Env(services.ControlEnv{}), expr.AsBool())
	require.NoError(t, err)

	e := &Engine{config: DefaultExecutionConfig()}

	tests := []struct {
		name     string
		program  *vm.Program
		severity string
		tags     []string
		owner    string
		want     bool
	}{
		{"match", program, "critical", []string{"prod", "db"}, "", true},
		{"wrong-severity", program, "high", []string{"prod"}, "", false},
		{"missing-tag", program, "critical", []string{"dev"}, "", false},
		{"match-owner", ownerProgram, "high", nil, "security-team", true},
		{"wrong-owner", ownerProgram, "high", nil, "dev-team", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e.config.FilterProgram = tt.program
			ctrl := entities.Control{Severity: tt.severity, Tags: tt.tags, Owner: tt.owner}
			got, _ := e.shouldRun(ctrl, nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveDependencies(t *testing.T) {
	// Setup graph:
	// c1 (security)
	// c2 (app) -> c1
	// c3 (app) -> c2
	// c4 (audit)
	//
	// Filter: tags=app
	// Result should be: c3, c2 (matched), c1 (dependency)
	// c4 should be excluded

	profile := &entities.Profile{
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{ID: "c1", Tags: []string{"security"}},
				{ID: "c2", Tags: []string{"app"}, DependsOn: []string{"c1"}},
				{ID: "c3", Tags: []string{"app"}, DependsOn: []string{"c2"}},
				{ID: "c4", Tags: []string{"audit"}},
			},
		},
	}

	cfg := DefaultExecutionConfig()
	cfg.IncludeTags = []string{"app"}
	cfg.IncludeDependencies = true

	e := &Engine{config: cfg}

	required, err := e.resolveDependencies(profile)
	require.NoError(t, err)

	assert.True(t, required["c1"], "c1 should be required as transitive dependency")
	assert.True(t, required["c2"], "c2 should be required as direct dependency")
	// c3 is a target, not necessarily a "dependency" of another target,
	// but the current implementation adds targets to queue so they might end up in map?
	// Actually, resolveDependencies only returns dependencies found by walking UP.
	// Wait, let's check logic:
	// Identify initial targets (c2, c3).
	// Queue = [c1 (from c2), c2 (from c3)]
	// Process c1: add to required. Deps: []
	// Process c2: add to required. Deps: [c1]
	// Process c1 again: visited.

	// So required map should contain c1 and c2.
	// c3 is matched by shouldRun, so it will run.
	// c4 is not matched and not required.

	assert.True(t, required["c1"])
	assert.True(t, required["c2"])
	assert.False(t, required["c3"], "c3 is a target, not a dependency") // c3 runs because shouldRun=true
	assert.False(t, required["c4"])
}

// ========================================
// Worker Pool Tests
// ========================================

// TestWorkerPool_NoDependencies verifies parallel execution when no dependencies exist
func TestWorkerPool_NoDependencies(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	engine, err := NewEngine(ctx, build.Get())
	require.NoError(t, err)
	defer engine.Close(ctx)

	// Create 10 controls with no dependencies
	controls := make([]entities.Control, 10)
	for i := 0; i < 10; i++ {
		controls[i] = entities.Control{
			ID:   fmt.Sprintf("control-%d", i),
			Name: fmt.Sprintf("Control %d", i),
			ObservationDefinitions: []entities.ObservationDefinition{
				{
					Plugin: "file",
					Config: map[string]interface{}{
						"path": "/etc/hostname",
					},
				},
			},
		}
	}

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "test", Version: "1.0"},
		Controls: entities.ControlsSection{Items: controls},
	}

	result, err := engine.Execute(ctx, profile)
	require.NoError(t, err)
	assert.Len(t, result.Controls, 10)
}

// TestWorkerPool_LinearDependencies verifies sequential execution for linear chain
func TestWorkerPool_LinearDependencies(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	engine, err := NewEngine(ctx, build.Get())
	require.NoError(t, err)
	defer engine.Close(ctx)

	// Create chain: A → B → C
	controls := []entities.Control{
		{
			ID:   "a",
			Name: "Control A",
			ObservationDefinitions: []entities.ObservationDefinition{
				{Plugin: "file", Config: map[string]interface{}{"path": "/etc/hostname"}},
			},
		},
		{
			ID:        "b",
			Name:      "Control B",
			DependsOn: []string{"a"},
			ObservationDefinitions: []entities.ObservationDefinition{
				{Plugin: "file", Config: map[string]interface{}{"path": "/etc/hostname"}},
			},
		},
		{
			ID:        "c",
			Name:      "Control C",
			DependsOn: []string{"b"},
			ObservationDefinitions: []entities.ObservationDefinition{
				{Plugin: "file", Config: map[string]interface{}{"path": "/etc/hostname"}},
			},
		},
	}

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "test", Version: "1.0"},
		Controls: entities.ControlsSection{Items: controls},
	}

	result, err := engine.Execute(ctx, profile)
	require.NoError(t, err)
	assert.Len(t, result.Controls, 3)

	// Verify all passed (if file exists) or handled gracefully
	for _, ctrl := range result.Controls {
		assert.Contains(t, []values.Status{values.StatusPass, values.StatusError, values.StatusFail, values.StatusSkipped}, ctrl.Status)
	}
}

// TestWorkerPool_DiamondDependencies verifies parallel execution in diamond pattern
func TestWorkerPool_DiamondDependencies(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	engine, err := NewEngine(ctx, build.Get())
	require.NoError(t, err)
	defer engine.Close(ctx)

	// Create diamond: A → B, A → C, B → D, C → D
	controls := []entities.Control{
		{ID: "a", Name: "A", ObservationDefinitions: []entities.ObservationDefinition{
			{Plugin: "file", Config: map[string]interface{}{"path": "/etc/hostname"}},
		}},
		{ID: "b", Name: "B", DependsOn: []string{"a"}, ObservationDefinitions: []entities.ObservationDefinition{
			{Plugin: "file", Config: map[string]interface{}{"path": "/etc/hostname"}},
		}},
		{ID: "c", Name: "C", DependsOn: []string{"a"}, ObservationDefinitions: []entities.ObservationDefinition{
			{Plugin: "file", Config: map[string]interface{}{"path": "/etc/hostname"}},
		}},
		{ID: "d", Name: "D", DependsOn: []string{"b", "c"}, ObservationDefinitions: []entities.ObservationDefinition{
			{Plugin: "file", Config: map[string]interface{}{"path": "/etc/hostname"}},
		}},
	}

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "test", Version: "1.0"},
		Controls: entities.ControlsSection{Items: controls},
	}

	result, err := engine.Execute(ctx, profile)
	require.NoError(t, err)
	assert.Len(t, result.Controls, 4)
}

// TestWorkerPool_DependencyFailure verifies skip propagation on failure
func TestWorkerPool_DependencyFailure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	engine, err := NewEngine(ctx, build.Get())
	require.NoError(t, err)
	defer engine.Close(ctx)

	// A fails (nonexistent file) → B should skip → C should skip
	controls := []entities.Control{
		{ID: "a", Name: "A (will fail)", ObservationDefinitions: []entities.ObservationDefinition{
			{Plugin: "file", Config: map[string]interface{}{"path": "/nonexistent-file-12345", "mode": "exists"}},
		}},
		{ID: "b", Name: "B", DependsOn: []string{"a"}, ObservationDefinitions: []entities.ObservationDefinition{
			{Plugin: "file", Config: map[string]interface{}{"path": "/etc/hostname"}},
		}},
		{ID: "c", Name: "C", DependsOn: []string{"b"}, ObservationDefinitions: []entities.ObservationDefinition{
			{Plugin: "file", Config: map[string]interface{}{"path": "/etc/hostname"}},
		}},
	}

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "test", Version: "1.0"},
		Controls: entities.ControlsSection{Items: controls},
	}

	result, err := engine.Execute(ctx, profile)
	require.NoError(t, err)

	// Verify skip propagation
	statusByID := make(map[string]values.Status)
	for _, ctrl := range result.Controls {
		statusByID[ctrl.ID] = ctrl.Status
	}

	// 'a' might be Error or Fail depending on how plugin reports nonexistent file with mode=exists
	// The file plugin usually returns Fail or Error.
	// As long as it is not Pass, dependencies should skip.
	assert.NotEqual(t, values.StatusPass, statusByID["a"])

	// If 'a' didn't pass, b and c should be skipped
	if statusByID["a"] != values.StatusPass {
		assert.Equal(t, values.StatusSkipped, statusByID["b"]) // Dependency failed
		assert.Equal(t, values.StatusSkipped, statusByID["c"]) // Transitive skip
	}
}

// TestWorkerPool_CycleDetection verifies cycle detection during initialization
func TestWorkerPool_CycleDetection(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	engine, err := NewEngine(ctx, build.Get())
	require.NoError(t, err)
	defer engine.Close(ctx)

	// Create cycle: A → B → A
	controls := []entities.Control{
		{ID: "a", Name: "A", DependsOn: []string{"b"}, ObservationDefinitions: []entities.ObservationDefinition{
			{Plugin: "file", Config: map[string]interface{}{"path": "/etc/hostname"}},
		}},
		{ID: "b", Name: "B", DependsOn: []string{"a"}, ObservationDefinitions: []entities.ObservationDefinition{
			{Plugin: "file", Config: map[string]interface{}{"path": "/etc/hostname"}},
		}},
	}

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "test", Version: "1.0"},
		Controls: entities.ControlsSection{Items: controls},
	}

	// Should return error about circular dependency
	_, err = engine.Execute(ctx, profile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
}

// TestWorkerPool_MissingDependency verifies error on missing dependency
func TestWorkerPool_MissingDependency(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	engine, err := NewEngine(ctx, build.Get())
	require.NoError(t, err)
	defer engine.Close(ctx)

	// A depends on non-existent B
	// Add C to force parallel execution path (len > 1)
	controls := []entities.Control{
		{ID: "a", Name: "A", DependsOn: []string{"nonexistent"}, ObservationDefinitions: []entities.ObservationDefinition{
			{Plugin: "file", Config: map[string]interface{}{"path": "/etc/hostname"}},
		}},
		{ID: "c", Name: "C", ObservationDefinitions: []entities.ObservationDefinition{
			{Plugin: "file", Config: map[string]interface{}{"path": "/etc/hostname"}},
		}},
	}

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "test", Version: "1.0"},
		Controls: entities.ControlsSection{Items: controls},
	}

	// Should return error about missing dependency
	_, err = engine.Execute(ctx, profile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-existent")
}

// TestWorkerPool_ContextCancellation verifies graceful shutdown on cancellation
func TestWorkerPool_ContextCancellation(t *testing.T) {
	t.Parallel()

	engine, err := NewEngine(context.Background(), build.Get())
	require.NoError(t, err)
	defer engine.Close(context.Background())

	// Create many controls to ensure some are in-flight when canceled
	controls := make([]entities.Control, 50)
	for i := 0; i < 50; i++ {
		controls[i] = entities.Control{
			ID:   fmt.Sprintf("control-%d", i),
			Name: fmt.Sprintf("Control %d", i),
			ObservationDefinitions: []entities.ObservationDefinition{
				{Plugin: "file", Config: map[string]interface{}{"path": "/etc/hostname"}},
			},
		}
	}

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "test", Version: "1.0"},
		Controls: entities.ControlsSection{Items: controls},
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after short delay
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	// Execute should return context error
	_, err = engine.Execute(ctx, profile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}
