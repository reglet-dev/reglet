// Package entities contains domain entities for the Reglet domain model.
// These are pure domain types with NO infrastructure dependencies.
package entities

import "runtime"

// ExecutionPlanLevel represents controls that can execute in parallel at a given level.
// Controls within the same level have no dependencies on each other.
type ExecutionPlanLevel struct {
	Controls []ControlSummary
	Level    int
}

// ControlSummary is a lightweight view of a control for planning purposes.
// It contains only the information needed to display an execution plan.
type ControlSummary struct {
	ID           string
	Name         string
	Severity     string
	DependsOn    []string
	Tags         []string
	Observations int
	Expectations int
}

// ExecutionPlan represents a dry-run execution plan showing which controls
// would run and in what order, without actually executing them.
type ExecutionPlan struct {
	ProfileName     string
	ProfileVersion  string
	Levels          []ExecutionPlanLevel
	TotalControls   int
	MaxParallelism  int
	HasDependencies bool
}

// NewExecutionPlan creates an ExecutionPlan from profile metadata and control levels.
// It calculates statistics like total controls, max parallelism, and dependency presence.
func NewExecutionPlan(name, version string, levels []ExecutionPlanLevel) *ExecutionPlan {
	totalControls := 0
	maxParallel := 0
	hasDeps := false

	for _, level := range levels {
		levelSize := len(level.Controls)
		totalControls += levelSize
		if levelSize > maxParallel {
			maxParallel = levelSize
		}
		for _, ctrl := range level.Controls {
			if len(ctrl.DependsOn) > 0 {
				hasDeps = true
			}
		}
	}

	// Cap at runtime CPU count for realistic parallelism estimate
	cpuCount := runtime.NumCPU()
	if maxParallel > cpuCount {
		maxParallel = cpuCount
	}

	return &ExecutionPlan{
		ProfileName:     name,
		ProfileVersion:  version,
		Levels:          levels,
		TotalControls:   totalControls,
		MaxParallelism:  maxParallel,
		HasDependencies: hasDeps,
	}
}

// LevelCount returns the number of execution levels in the plan.
func (p *ExecutionPlan) LevelCount() int {
	return len(p.Levels)
}

// IsEmpty returns true if the plan contains no controls.
func (p *ExecutionPlan) IsEmpty() bool {
	return p.TotalControls == 0
}

// ControlSummaryFromControl creates a ControlSummary from a Control entity.
func ControlSummaryFromControl(ctrl Control) ControlSummary {
	// Count total expectations across all observations
	expectCount := 0
	for _, obs := range ctrl.ObservationDefinitions {
		expectCount += len(obs.Expect)
	}

	return ControlSummary{
		ID:           ctrl.ID,
		Name:         ctrl.Name,
		Severity:     ctrl.Severity,
		DependsOn:    ctrl.DependsOn,
		Observations: len(ctrl.ObservationDefinitions),
		Expectations: expectCount,
		Tags:         ctrl.Tags,
	}
}
