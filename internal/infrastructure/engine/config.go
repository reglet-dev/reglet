// Package engine coordinates profile execution and validation.
package engine

import (
	"runtime"

	"github.com/expr-lang/expr/vm"
)

// Concurrency constants for parallel execution.
const (
	// MinConcurrentControls is the minimum number of concurrent control executions,
	// ensuring reasonable parallelism even on single-core systems.
	MinConcurrentControls = 4

	// MaxConcurrentObservations caps the per-control observation parallelism
	// to avoid excessive goroutine nesting.
	MaxConcurrentObservations = 10

	// MinConcurrentObservations ensures reasonable parallelism for observations.
	MinConcurrentObservations = 2
)

// ExecutionConfig controls execution behavior.
type ExecutionConfig struct {
	// MaxConcurrentControls limits parallel control execution (0 = no limit)
	MaxConcurrentControls int
	// MaxConcurrentObservations limits parallel observation execution within a control (0 = no limit)
	MaxConcurrentObservations int
	// Parallel enables parallel execution (default: true for performance)
	Parallel bool

	// Include Filters (OR logic within slice, AND between types)
	IncludeTags       []string
	IncludeSeverities []string
	IncludeControlIDs []string // Exclusive - if set, other filters ignored

	// Exclude Filters (take precedence over includes)
	ExcludeTags       []string
	ExcludeControlIDs []string

	// Advanced Filter (Compiled Expression)
	FilterProgram *vm.Program

	// Dependency Strategy
	IncludeDependencies bool
}

// DefaultExecutionConfig returns sensible defaults for parallel execution.
func DefaultExecutionConfig() ExecutionConfig {
	numCPU := runtime.NumCPU()

	// Default to NumCPU for controls, but at least MinConcurrentControls
	maxControls := numCPU
	if maxControls < MinConcurrentControls {
		maxControls = MinConcurrentControls
	}

	// Observations are within a control, so we use a smaller multiple of NumCPU
	maxObs := numCPU / 2
	if maxObs < MinConcurrentObservations {
		maxObs = MinConcurrentObservations
	}
	if maxObs > MaxConcurrentObservations {
		maxObs = MaxConcurrentObservations
	}

	return ExecutionConfig{
		MaxConcurrentControls:     maxControls,
		MaxConcurrentObservations: maxObs,
		Parallel:                  true,
	}
}
