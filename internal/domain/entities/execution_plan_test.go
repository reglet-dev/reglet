package entities

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewExecutionPlan_CalculatesStatistics(t *testing.T) {
	t.Parallel()

	levels := []ExecutionPlanLevel{
		{
			Level: 0,
			Controls: []ControlSummary{
				{ID: "ctrl-1", Name: "Control 1", Observations: 2},
				{ID: "ctrl-2", Name: "Control 2", Observations: 1},
			},
		},
		{
			Level: 1,
			Controls: []ControlSummary{
				{ID: "ctrl-3", Name: "Control 3", DependsOn: []string{"ctrl-1"}, Observations: 3},
			},
		},
	}

	plan := NewExecutionPlan("test-profile", "1.0.0", levels)

	require.NotNil(t, plan)
	assert.Equal(t, "test-profile", plan.ProfileName)
	assert.Equal(t, "1.0.0", plan.ProfileVersion)
	assert.Equal(t, 3, plan.TotalControls)
	assert.Equal(t, 2, plan.LevelCount())
	assert.True(t, plan.HasDependencies)

	// MaxParallelism should be capped at CPU count
	expectedParallel := 2
	if expectedParallel > runtime.NumCPU() {
		expectedParallel = runtime.NumCPU()
	}
	assert.Equal(t, expectedParallel, plan.MaxParallelism)
}

func TestNewExecutionPlan_NoDependencies(t *testing.T) {
	t.Parallel()

	levels := []ExecutionPlanLevel{
		{
			Level: 0,
			Controls: []ControlSummary{
				{ID: "ctrl-1", Name: "Control 1"},
				{ID: "ctrl-2", Name: "Control 2"},
				{ID: "ctrl-3", Name: "Control 3"},
			},
		},
	}

	plan := NewExecutionPlan("flat-profile", "2.0.0", levels)

	require.NotNil(t, plan)
	assert.False(t, plan.HasDependencies)
	assert.Equal(t, 1, plan.LevelCount())
	assert.Equal(t, 3, plan.TotalControls)
}

func TestNewExecutionPlan_Empty(t *testing.T) {
	t.Parallel()

	plan := NewExecutionPlan("empty-profile", "1.0.0", nil)

	require.NotNil(t, plan)
	assert.True(t, plan.IsEmpty())
	assert.Equal(t, 0, plan.TotalControls)
	assert.Equal(t, 0, plan.LevelCount())
	assert.Equal(t, 0, plan.MaxParallelism)
	assert.False(t, plan.HasDependencies)
}

func TestExecutionPlan_LevelCount(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		levels   []ExecutionPlanLevel
		expected int
	}{
		{"no levels", nil, 0},
		{"one level", []ExecutionPlanLevel{{Level: 0}}, 1},
		{"three levels", []ExecutionPlanLevel{{Level: 0}, {Level: 1}, {Level: 2}}, 3},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			plan := NewExecutionPlan("test", "1.0", tc.levels)
			assert.Equal(t, tc.expected, plan.LevelCount())
		})
	}
}

func TestControlSummaryFromControl(t *testing.T) {
	t.Parallel()

	ctrl := Control{
		ID:        "test-ctrl",
		Name:      "Test Control",
		Severity:  "high",
		DependsOn: []string{"other-ctrl"},
		Tags:      []string{"security", "compliance"},
		ObservationDefinitions: []ObservationDefinition{
			{Plugin: "file", Expect: []string{"data.exists", "data.readable"}},
			{Plugin: "http", Expect: []string{"data.status_code == 200"}},
		},
	}

	summary := ControlSummaryFromControl(ctrl)

	assert.Equal(t, ctrl.ID, summary.ID)
	assert.Equal(t, ctrl.Name, summary.Name)
	assert.Equal(t, ctrl.Severity, summary.Severity)
	assert.Equal(t, ctrl.DependsOn, summary.DependsOn)
	assert.Equal(t, ctrl.Tags, summary.Tags)
	assert.Equal(t, 2, summary.Observations)
	assert.Equal(t, 3, summary.Expectations) // 2 from first obs + 1 from second obs
}
