package engine

import (
	"testing"

	"github.com/jrose/reglet/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildControlDAG_NoDependencies(t *testing.T) {
	controls := []config.Control{
		{ID: "a"},
		{ID: "b"},
		{ID: "c"},
	}

	levels, err := BuildControlDAG(controls)
	require.NoError(t, err)
	require.Len(t, levels, 1)

	// All controls should be in level 0
	assert.Equal(t, 0, levels[0].Level)
	assert.Len(t, levels[0].Controls, 3)

	// Verify all control IDs are present
	ids := make(map[string]bool)
	for _, ctrl := range levels[0].Controls {
		ids[ctrl.ID] = true
	}
	assert.True(t, ids["a"])
	assert.True(t, ids["b"])
	assert.True(t, ids["c"])
}

func TestBuildControlDAG_LinearDependency(t *testing.T) {
	controls := []config.Control{
		{ID: "a"},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"b"}},
	}

	levels, err := BuildControlDAG(controls)
	require.NoError(t, err)
	require.Len(t, levels, 3)

	// Level 0: a
	assert.Equal(t, 0, levels[0].Level)
	assert.Len(t, levels[0].Controls, 1)
	assert.Equal(t, "a", levels[0].Controls[0].ID)

	// Level 1: b
	assert.Equal(t, 1, levels[1].Level)
	assert.Len(t, levels[1].Controls, 1)
	assert.Equal(t, "b", levels[1].Controls[0].ID)

	// Level 2: c
	assert.Equal(t, 2, levels[2].Level)
	assert.Len(t, levels[2].Controls, 1)
	assert.Equal(t, "c", levels[2].Controls[0].ID)
}

func TestBuildControlDAG_DiamondDependency(t *testing.T) {
	controls := []config.Control{
		{ID: "a"},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"a"}},
		{ID: "d", DependsOn: []string{"b", "c"}},
	}

	levels, err := BuildControlDAG(controls)
	require.NoError(t, err)
	require.Len(t, levels, 3)

	// Level 0: a
	assert.Equal(t, 0, levels[0].Level)
	assert.Len(t, levels[0].Controls, 1)
	assert.Equal(t, "a", levels[0].Controls[0].ID)

	// Level 1: b and c (can run in parallel)
	assert.Equal(t, 1, levels[1].Level)
	assert.Len(t, levels[1].Controls, 2)
	ids := make(map[string]bool)
	for _, ctrl := range levels[1].Controls {
		ids[ctrl.ID] = true
	}
	assert.True(t, ids["b"])
	assert.True(t, ids["c"])

	// Level 2: d
	assert.Equal(t, 2, levels[2].Level)
	assert.Len(t, levels[2].Controls, 1)
	assert.Equal(t, "d", levels[2].Controls[0].ID)
}

func TestBuildControlDAG_ComplexDependency(t *testing.T) {
	// More complex DAG:
	//       a
	//      / \
	//     b   c
	//     |   |\
	//     d   e f
	//      \ /
	//       g
	controls := []config.Control{
		{ID: "a"},
		{ID: "b", DependsOn: []string{"a"}},
		{ID: "c", DependsOn: []string{"a"}},
		{ID: "d", DependsOn: []string{"b"}},
		{ID: "e", DependsOn: []string{"c"}},
		{ID: "f", DependsOn: []string{"c"}},
		{ID: "g", DependsOn: []string{"d", "e"}},
	}

	levels, err := BuildControlDAG(controls)
	require.NoError(t, err)
	require.Len(t, levels, 4)

	// Level 0: a
	assert.Equal(t, 0, levels[0].Level)
	assert.Len(t, levels[0].Controls, 1)

	// Level 1: b, c
	assert.Equal(t, 1, levels[1].Level)
	assert.Len(t, levels[1].Controls, 2)

	// Level 2: d, e, f
	assert.Equal(t, 2, levels[2].Level)
	assert.Len(t, levels[2].Controls, 3)

	// Level 3: g
	assert.Equal(t, 3, levels[3].Level)
	assert.Len(t, levels[3].Controls, 1)
	assert.Equal(t, "g", levels[3].Controls[0].ID)
}

func TestBuildControlDAG_CircularDependency(t *testing.T) {
	controls := []config.Control{
		{ID: "a", DependsOn: []string{"b"}},
		{ID: "b", DependsOn: []string{"a"}},
	}

	_, err := BuildControlDAG(controls)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
}

func TestBuildControlDAG_CircularDependency_ThreeNodes(t *testing.T) {
	controls := []config.Control{
		{ID: "a", DependsOn: []string{"b"}},
		{ID: "b", DependsOn: []string{"c"}},
		{ID: "c", DependsOn: []string{"a"}},
	}

	_, err := BuildControlDAG(controls)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
}

func TestBuildControlDAG_MissingDependency(t *testing.T) {
	controls := []config.Control{
		{ID: "a", DependsOn: []string{"nonexistent"}},
	}

	_, err := BuildControlDAG(controls)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-existent control")
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestBuildControlDAG_SelfDependency(t *testing.T) {
	controls := []config.Control{
		{ID: "a", DependsOn: []string{"a"}},
	}

	_, err := BuildControlDAG(controls)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
}

func TestBuildControlDAG_MultipleDependencies(t *testing.T) {
	controls := []config.Control{
		{ID: "a"},
		{ID: "b"},
		{ID: "c", DependsOn: []string{"a", "b"}},
	}

	levels, err := BuildControlDAG(controls)
	require.NoError(t, err)
	require.Len(t, levels, 2)

	// Level 0: a and b (can run in parallel)
	assert.Equal(t, 0, levels[0].Level)
	assert.Len(t, levels[0].Controls, 2)

	// Level 1: c (waits for both a and b)
	assert.Equal(t, 1, levels[1].Level)
	assert.Len(t, levels[1].Controls, 1)
	assert.Equal(t, "c", levels[1].Controls[0].ID)
}

func TestBuildControlDAG_EmptyControls(t *testing.T) {
	controls := []config.Control{}

	levels, err := BuildControlDAG(controls)
	require.NoError(t, err)
	assert.Len(t, levels, 0)
}

func TestBuildControlDAG_SingleControl(t *testing.T) {
	controls := []config.Control{
		{ID: "a"},
	}

	levels, err := BuildControlDAG(controls)
	require.NoError(t, err)
	require.Len(t, levels, 1)
	assert.Equal(t, 0, levels[0].Level)
	assert.Len(t, levels[0].Controls, 1)
	assert.Equal(t, "a", levels[0].Controls[0].ID)
}
