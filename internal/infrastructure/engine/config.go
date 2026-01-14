// Package engine coordinates profile execution and validation.
package engine

import (
	"runtime"

	"github.com/expr-lang/expr/vm"
	"github.com/reglet-dev/reglet/internal/domain/constants"
)

// Concurrency constants for parallel execution (re-exported from constants package).
const (
	// MinConcurrentControls is the minimum number of concurrent control executions.
	MinConcurrentControls = constants.DefaultMinConcurrentControls

	// MaxConcurrentObservations caps the per-control observation parallelism.
	MaxConcurrentObservations = constants.DefaultMaxConcurrentObservations

	// MinConcurrentObservations ensures reasonable parallelism for observations.
	MinConcurrentObservations = constants.DefaultMinConcurrentObservations
)

// ExecutionConfig controls execution behavior.
type ExecutionConfig struct {
	FilterProgram     *vm.Program
	IncludeTags       []string
	IncludeSeverities []string
	IncludeControlIDs []string
	ExcludeTags       []string
	ExcludeControlIDs []string

	MaxConcurrentControls     int
	MaxConcurrentObservations int
	MaxEvidenceSizeBytes      int

	Parallel            bool
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
		MaxEvidenceSizeBytes:      0, // 0 = no limit (or use default from business logic)
	}
}
