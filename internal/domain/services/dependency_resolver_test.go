package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
)

func Test_DependencyResolver_BuildControlDAG_NoDependencies(t *testing.T) {
	resolver := NewDependencyResolver()
	controls := []entities.Control{
		{ID: "ctrl-1"},
		{ID: "ctrl-2"},
		{ID: "ctrl-3"},
	}

	levels, err := resolver.BuildControlDAG(controls)
	require.NoError(t, err)
	require.Len(t, levels, 1, "all controls should be in level 0")
	assert.Equal(t, 0, levels[0].Level)
	assert.Len(t, levels[0].Controls, 3)
}

func Test_DependencyResolver_BuildControlDAG_LinearDependencies(t *testing.T) {
	resolver := NewDependencyResolver()
	controls := []entities.Control{
		{ID: "ctrl-1"},
		{ID: "ctrl-2", DependsOn: []string{"ctrl-1"}},
		{ID: "ctrl-3", DependsOn: []string{"ctrl-2"}},
	}

	levels, err := resolver.BuildControlDAG(controls)
	require.NoError(t, err)
	require.Len(t, levels, 3)

	// Level 0: ctrl-1
	assert.Equal(t, 0, levels[0].Level)
	assert.Len(t, levels[0].Controls, 1)
	assert.Equal(t, "ctrl-1", levels[0].Controls[0].ID)

	// Level 1: ctrl-2
	assert.Equal(t, 1, levels[1].Level)
	assert.Len(t, levels[1].Controls, 1)
	assert.Equal(t, "ctrl-2", levels[1].Controls[0].ID)

	// Level 2: ctrl-3
	assert.Equal(t, 2, levels[2].Level)
	assert.Len(t, levels[2].Controls, 1)
	assert.Equal(t, "ctrl-3", levels[2].Controls[0].ID)
}

func Test_DependencyResolver_BuildControlDAG_ParallelExecution(t *testing.T) {
	resolver := NewDependencyResolver()
	controls := []entities.Control{
		{ID: "ctrl-base"},
		{ID: "ctrl-a", DependsOn: []string{"ctrl-base"}},
		{ID: "ctrl-b", DependsOn: []string{"ctrl-base"}},
		{ID: "ctrl-c", DependsOn: []string{"ctrl-base"}},
	}

	levels, err := resolver.BuildControlDAG(controls)
	require.NoError(t, err)
	require.Len(t, levels, 2)

	// Level 0: ctrl-base
	assert.Equal(t, 0, levels[0].Level)
	assert.Len(t, levels[0].Controls, 1)
	assert.Equal(t, "ctrl-base", levels[0].Controls[0].ID)

	// Level 1: ctrl-a, ctrl-b, ctrl-c (can run in parallel)
	assert.Equal(t, 1, levels[1].Level)
	assert.Len(t, levels[1].Controls, 3)
}

func Test_DependencyResolver_BuildControlDAG_CircularDependency(t *testing.T) {
	resolver := NewDependencyResolver()
	controls := []entities.Control{
		{ID: "ctrl-1", DependsOn: []string{"ctrl-2"}},
		{ID: "ctrl-2", DependsOn: []string{"ctrl-1"}},
	}

	_, err := resolver.BuildControlDAG(controls)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
}

func Test_DependencyResolver_BuildControlDAG_NonExistentDependency(t *testing.T) {
	resolver := NewDependencyResolver()
	controls := []entities.Control{
		{ID: "ctrl-1", DependsOn: []string{"ctrl-nonexistent"}},
	}

	_, err := resolver.BuildControlDAG(controls)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-existent control")
}

func Test_DependencyResolver_ResolveDependencies_NoDeps(t *testing.T) {
	resolver := NewDependencyResolver()
	controls := []entities.Control{
		{ID: "ctrl-1"},
		{ID: "ctrl-2"},
	}

	deps, err := resolver.ResolveDependencies(controls)
	require.NoError(t, err)
	assert.Len(t, deps["ctrl-1"], 0, "ctrl-1 has no dependencies")
	assert.Len(t, deps["ctrl-2"], 0, "ctrl-2 has no dependencies")
}

func Test_DependencyResolver_ResolveDependencies_DirectDeps(t *testing.T) {
	resolver := NewDependencyResolver()
	controls := []entities.Control{
		{ID: "ctrl-1"},
		{ID: "ctrl-2", DependsOn: []string{"ctrl-1"}},
	}

	deps, err := resolver.ResolveDependencies(controls)
	require.NoError(t, err)
	assert.Len(t, deps["ctrl-1"], 0)
	assert.Len(t, deps["ctrl-2"], 1)
	assert.True(t, deps["ctrl-2"]["ctrl-1"], "ctrl-2 depends on ctrl-1")
}

func Test_DependencyResolver_ResolveDependencies_TransitiveDeps(t *testing.T) {
	resolver := NewDependencyResolver()
	controls := []entities.Control{
		{ID: "ctrl-1"},
		{ID: "ctrl-2", DependsOn: []string{"ctrl-1"}},
		{ID: "ctrl-3", DependsOn: []string{"ctrl-2"}},
	}

	deps, err := resolver.ResolveDependencies(controls)
	require.NoError(t, err)

	// ctrl-3 should have both ctrl-2 and ctrl-1 as dependencies
	assert.Len(t, deps["ctrl-3"], 2)
	assert.True(t, deps["ctrl-3"]["ctrl-2"], "ctrl-3 depends on ctrl-2")
	assert.True(t, deps["ctrl-3"]["ctrl-1"], "ctrl-3 transitively depends on ctrl-1")
}

func Test_DependencyResolver_ResolveDependencies_CircularDependency(t *testing.T) {
	resolver := NewDependencyResolver()
	controls := []entities.Control{
		{ID: "ctrl-1", DependsOn: []string{"ctrl-2"}},
		{ID: "ctrl-2", DependsOn: []string{"ctrl-1"}},
	}

	_, err := resolver.ResolveDependencies(controls)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
}
