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
	FilterProgram             *vm.Program
	IncludeTags               []string
	IncludeSeverities         []string
	IncludeControlIDs         []string
	ExcludeTags               []string
	ExcludeControlIDs         []string
	MaxConcurrentControls     int
	MaxConcurrentObservations int
	Parallel                  bool
	IncludeDependencies       bool
	MaxEvidenceSizeBytes      int
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
		MaxEvidenceSizeBytes:      0, // 0 = no limit (or use default from business logic)
	}
}
