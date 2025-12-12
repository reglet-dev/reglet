package services

import (
	"testing"

	"github.com/expr-lang/expr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/whiskeyjimbo/reglet/internal/config"
)

func Test_ControlFilter_NoFilters(t *testing.T) {
	filter := NewControlFilter()
	ctrl := config.Control{ID: "ctrl-1"}

	shouldRun, _ := filter.ShouldRun(ctrl)
	assert.True(t, shouldRun, "no filters should allow all controls")
}

func Test_ControlFilter_ExclusiveMode(t *testing.T) {
	filter := NewControlFilter().
		WithExclusiveControls([]string{"ctrl-1", "ctrl-2"})

	tests := []struct {
		controlID string
		expected  bool
	}{
		{"ctrl-1", true},
		{"ctrl-2", true},
		{"ctrl-3", false},
	}

	for _, tt := range tests {
		t.Run(tt.controlID, func(t *testing.T) {
			ctrl := config.Control{ID: tt.controlID}
			shouldRun, _ := filter.ShouldRun(ctrl)
			assert.Equal(t, tt.expected, shouldRun)
		})
	}
}

func Test_ControlFilter_ExcludeControlIDs(t *testing.T) {
	filter := NewControlFilter().
		WithExcludedControls([]string{"ctrl-exclude"})

	tests := []struct {
		controlID string
		expected  bool
	}{
		{"ctrl-1", true},
		{"ctrl-exclude", false},
	}

	for _, tt := range tests {
		t.Run(tt.controlID, func(t *testing.T) {
			ctrl := config.Control{ID: tt.controlID}
			shouldRun, _ := filter.ShouldRun(ctrl)
			assert.Equal(t, tt.expected, shouldRun)
		})
	}
}

func Test_ControlFilter_ExcludeTags(t *testing.T) {
	filter := NewControlFilter().
		WithExcludedTags([]string{"skip", "disabled"})

	tests := []struct {
		name     string
		tags     []string
		expected bool
	}{
		{"no tags", []string{}, true},
		{"allowed tags", []string{"production", "security"}, true},
		{"excluded tag", []string{"skip"}, false},
		{"mix of tags", []string{"production", "disabled"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := config.Control{ID: "ctrl-1", Tags: tt.tags}
			shouldRun, _ := filter.ShouldRun(ctrl)
			assert.Equal(t, tt.expected, shouldRun)
		})
	}
}

func Test_ControlFilter_IncludeTags(t *testing.T) {
	filter := NewControlFilter().
		WithIncludedTags([]string{"production", "security"})

	tests := []struct {
		name     string
		tags     []string
		expected bool
	}{
		{"no tags", []string{}, false},
		{"matching tag", []string{"production"}, true},
		{"other tag", []string{"development"}, false},
		{"multiple with match", []string{"development", "security"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := config.Control{ID: "ctrl-1", Tags: tt.tags}
			shouldRun, _ := filter.ShouldRun(ctrl)
			assert.Equal(t, tt.expected, shouldRun)
		})
	}
}

func Test_ControlFilter_IncludeSeverities(t *testing.T) {
	filter := NewControlFilter().
		WithIncludedSeverities([]string{"critical", "high"})

	tests := []struct {
		severity string
		expected bool
	}{
		{"critical", true},
		{"high", true},
		{"medium", false},
		{"low", false},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			ctrl := config.Control{ID: "ctrl-1", Severity: tt.severity}
			shouldRun, _ := filter.ShouldRun(ctrl)
			assert.Equal(t, tt.expected, shouldRun)
		})
	}
}

func Test_ControlFilter_FilterExpression(t *testing.T) {
	// Compile expression: owner == "platform"
	program, err := expr.Compile("owner == \"platform\"", expr.Env(ControlEnv{}), expr.AsBool())
	require.NoError(t, err)

	filter := NewControlFilter().
		WithFilterExpression(program)

	tests := []struct {
		owner    string
		expected bool
	}{
		{"platform", true},
		{"security", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.owner, func(t *testing.T) {
			ctrl := config.Control{ID: "ctrl-1", Owner: tt.owner}
			shouldRun, _ := filter.ShouldRun(ctrl)
			assert.Equal(t, tt.expected, shouldRun)
		})
	}
}

func Test_ControlFilter_Precedence(t *testing.T) {
	// Test: Exclusive mode overrides all other filters
	filter := NewControlFilter().
		WithExclusiveControls([]string{"ctrl-1"}).
		WithIncludedTags([]string{"production"}). // This should be ignored
		WithIncludedSeverities([]string{"high"})  // This should be ignored

	// ctrl-2 has matching tags/severity but NOT in exclusive list
	ctrl := config.Control{
		ID:       "ctrl-2",
		Tags:     []string{"production"},
		Severity: "high",
	}

	shouldRun, _ := filter.ShouldRun(ctrl)
	assert.False(t, shouldRun, "exclusive mode should override include filters")
}

func Test_ControlFilter_CombinedFilters(t *testing.T) {
	filter := NewControlFilter().
		WithExcludedTags([]string{"disabled"}).
		WithIncludedSeverities([]string{"critical", "high"})

	tests := []struct {
		name     string
		ctrl     config.Control
		expected bool
	}{
		{
			"high severity, no disabled tag",
			config.Control{ID: "1", Severity: "high", Tags: []string{"production"}},
			true,
		},
		{
			"critical severity, disabled tag",
			config.Control{ID: "2", Severity: "critical", Tags: []string{"disabled"}},
			false,
		},
		{
			"medium severity, no disabled tag",
			config.Control{ID: "3", Severity: "medium", Tags: []string{"production"}},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldRun, _ := filter.ShouldRun(tt.ctrl)
			assert.Equal(t, tt.expected, shouldRun)
		})
	}
}