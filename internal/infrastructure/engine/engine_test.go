package engine

import (
	"context"
	"testing"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/whiskeyjimbo/reglet/internal/domain"
	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
	"github.com/whiskeyjimbo/reglet/internal/domain/execution"
	"github.com/whiskeyjimbo/reglet/internal/domain/services"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/wasm"
)

func TestNewEngine(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	engine, err := NewEngine(ctx)
	require.NoError(t, err)
	require.NotNil(t, engine)
	require.NotNil(t, engine.runtime)
	require.NotNil(t, engine.executor)

	// Cleanup
	err = engine.Close(ctx)
	assert.NoError(t, err)
}

func TestGenerateControlMessage_SinglePass(t *testing.T) {
	t.Parallel()
	observations := []execution.ObservationResult{
		{Status: domain.StatusPass},
	}

	msg := generateControlMessage(domain.StatusPass, observations)
	assert.Equal(t, "Check passed", msg)
}

func TestGenerateControlMessage_MultiplePass(t *testing.T) {
	t.Parallel()
	observations := []execution.ObservationResult{
		{Status: domain.StatusPass},
		{Status: domain.StatusPass},
		{Status: domain.StatusPass},
	}

	msg := generateControlMessage(domain.StatusPass, observations)
	assert.Equal(t, "All 3 checks passed", msg)
}

func TestGenerateControlMessage_SingleFail(t *testing.T) {
	t.Parallel()
	observations := []execution.ObservationResult{
		{Status: domain.StatusPass},
		{Status: domain.StatusFail},
	}

	msg := generateControlMessage(domain.StatusFail, observations)
	assert.Equal(t, "1 check failed", msg)
}

func TestGenerateControlMessage_MultipleFail(t *testing.T) {
	t.Parallel()
	observations := []execution.ObservationResult{
		{Status: domain.StatusFail},
		{Status: domain.StatusFail},
		{Status: domain.StatusPass},
	}

	msg := generateControlMessage(domain.StatusFail, observations)
	assert.Equal(t, "2 checks failed", msg)
}

func TestGenerateControlMessage_SingleError(t *testing.T) {
	t.Parallel()
	observations := []execution.ObservationResult{
		{
			Status: domain.StatusError,
			Error:  &wasm.PluginError{Code: "test", Message: "something went wrong"},
		},
	}

	msg := generateControlMessage(domain.StatusError, observations)
	assert.Equal(t, "something went wrong", msg)
}

func TestGenerateControlMessage_SingleErrorNoMessage(t *testing.T) {
	t.Parallel()
	observations := []execution.ObservationResult{
		{
			Status: domain.StatusError,
			Error:  nil, // No error object
		},
	}

	msg := generateControlMessage(domain.StatusError, observations)
	assert.Equal(t, "Check encountered an error", msg)
}

func TestGenerateControlMessage_MultipleErrors(t *testing.T) {
	t.Parallel()
	observations := []execution.ObservationResult{
		{Status: domain.StatusError, Error: &wasm.PluginError{Code: "test", Message: "error 1"}},
		{Status: domain.StatusError, Error: &wasm.PluginError{Code: "test", Message: "error 2"}},
		{Status: domain.StatusPass},
	}

	msg := generateControlMessage(domain.StatusError, observations)
	assert.Equal(t, "2 checks encountered errors", msg)
}

func TestExecuteControl_SingleObservation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine, err := NewEngine(ctx)
	require.NoError(t, err)
	defer engine.Close(ctx)

	ctrl := entities.Control{
		ID:          "test-control",
		Name:        "Test Control",
		Description: "A test control",
		Severity:    "medium",
		Tags:        []string{"test"},
		Observations: []entities.Observation{
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
	result := engine.executeControl(ctx, ctrl, execResult, nil)

	assert.Equal(t, "test-control", result.ID)
	assert.Equal(t, "Test Control", result.Name)
	assert.Equal(t, "A test control", result.Description)
	assert.Equal(t, "medium", result.Severity)
	assert.Equal(t, []string{"test"}, result.Tags)
	assert.Len(t, result.Observations, 1)
	assert.Greater(t, result.Duration, time.Duration(0))
	assert.NotEmpty(t, result.Message)
}

func TestExecuteControl_MultipleObservations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine, err := NewEngine(ctx)
	require.NoError(t, err)
	defer engine.Close(ctx)

	ctrl := entities.Control{
		ID:   "multi-test",
		Name: "Multi Observation Test",
		Observations: []entities.Observation{
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
	result := engine.executeControl(ctx, ctrl, execResult, nil)

	assert.Equal(t, "multi-test", result.ID)
	assert.Len(t, result.Observations, 2)
	assert.Greater(t, result.Duration, time.Duration(0))
}

func TestExecute_SingleControl(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine, err := NewEngine(ctx)
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
					Observations: []entities.Observation{
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
	engine, err := NewEngine(ctx)
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
					Observations: []entities.Observation{
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
					Observations: []entities.Observation{
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
	engine, err := NewEngine(ctx)
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
					Observations: []entities.Observation{
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
	engine, err := NewEngine(ctx)
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
					Observations: []entities.Observation{
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
	assert.Greater(t, result.Controls[0].Observations[0].Duration, time.Duration(0))
}

func TestExecute_InvalidPlugin(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine, err := NewEngine(ctx)
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
					Observations: []entities.Observation{
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
	assert.Equal(t, domain.StatusError, result.Controls[0].Status)
	assert.Len(t, result.Controls[0].Observations, 1)
	assert.Equal(t, domain.StatusError, result.Controls[0].Observations[0].Status)
	assert.NotNil(t, result.Controls[0].Observations[0].Error)
	assert.Contains(t, result.Controls[0].Observations[0].Error.Message, "failed to read plugin")
}

// --- Filtering Tests ---

func TestShouldRun_IncludeTags(t *testing.T) {
	cfg := DefaultExecutionConfig()
	cfg.IncludeTags = []string{"security", "prod"}

	e := &Engine{config: cfg}

	tests := []struct {
		name string
		tags []string
		want bool
	}{
		{"match-security", []string{"security"}, true},
		{"match-prod", []string{"prod"}, true},
		{"match-both", []string{"security", "prod"}, true},
		{"match-mixed", []string{"security", "other"}, true},
		{"no-match", []string{"dev"}, false},
		{"no-tags", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := entities.Control{Tags: tt.tags}
			got, _ := e.shouldRun(ctrl)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestShouldRun_ExcludeTags(t *testing.T) {
	cfg := DefaultExecutionConfig()
	cfg.ExcludeTags = []string{"slow", "experimental"}

	e := &Engine{config: cfg}

	tests := []struct {
		name string
		tags []string
		want bool
	}{
		{"no-exclude", []string{"security"}, true},
		{"exclude-slow", []string{"slow"}, false},
		{"exclude-experimental", []string{"experimental"}, false},
		{"exclude-mixed", []string{"security", "slow"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := entities.Control{Tags: tt.tags}
			got, _ := e.shouldRun(ctrl)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestShouldRun_IncludeSeverity(t *testing.T) {
	cfg := DefaultExecutionConfig()
	cfg.IncludeSeverities = []string{"critical", "high"}

	e := &Engine{config: cfg}

	tests := []struct {
		name     string
		severity string
		want     bool
	}{
		{"match-critical", "critical", true},
		{"match-high", "high", true},
		{"no-match-low", "low", false},
		{"no-match-medium", "medium", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := entities.Control{Severity: tt.severity}
			got, _ := e.shouldRun(ctrl)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestShouldRun_IncludeControlIDs(t *testing.T) {
	cfg := DefaultExecutionConfig()
	cfg.IncludeControlIDs = []string{"c1", "c2"}
	// Other filters should be ignored when ControlIDs are present (exclusive mode)
	cfg.IncludeTags = []string{"ignored"}

	e := &Engine{config: cfg}

	tests := []struct {
		name string
		id   string
		tags []string
		want bool
	}{
		{"match-c1", "c1", []string{}, true},
		{"match-c2", "c2", []string{}, true},
		{"no-match-c3", "c3", []string{}, false},
		{"ignore-tags-match", "c1", []string{"ignored"}, true},
		{"ignore-tags-no-match", "c3", []string{"ignored"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := entities.Control{ID: tt.id, Tags: tt.tags}
			got, _ := e.shouldRun(ctrl)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestShouldRun_ExcludeControlIDs(t *testing.T) {
	cfg := DefaultExecutionConfig()
	cfg.ExcludeControlIDs = []string{"c1"}

	e := &Engine{config: cfg}

	tests := []struct {
		name string
		id   string
		want bool
	}{
		{"no-exclude", "c2", true},
		{"exclude-c1", "c1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := entities.Control{ID: tt.id}
			got, _ := e.shouldRun(ctrl)
			assert.Equal(t, tt.want, got)
		})
	}
}

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
			got, _ := e.shouldRun(ctrl)
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
