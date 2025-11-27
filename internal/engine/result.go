// Package engine provides profile execution orchestration for Reglet.
// It coordinates plugin execution and aggregates results.
package engine

import (
	"sync"
	"time"

	"github.com/jrose/reglet/internal/wasm"
)

// Status represents the status of a control or observation.
type Status string

const (
	// StatusPass indicates the check passed
	StatusPass Status = "pass"
	// StatusFail indicates the check failed (but ran successfully)
	StatusFail Status = "fail"
	// StatusError indicates the check encountered an error
	StatusError Status = "error"
)

// ExecutionResult represents the complete result of executing a profile.
type ExecutionResult struct {
	ProfileName    string          `json:"profile_name" yaml:"profile_name"`
	ProfileVersion string          `json:"profile_version" yaml:"profile_version"`
	StartTime      time.Time       `json:"start_time" yaml:"start_time"`
	EndTime        time.Time       `json:"end_time" yaml:"end_time"`
	Duration       time.Duration   `json:"duration_ms" yaml:"duration_ms"`
	Controls       []ControlResult `json:"controls" yaml:"controls"`
	Summary        ResultSummary   `json:"summary" yaml:"summary"`
	mu             sync.Mutex      // Protects Controls for concurrent AddControlResult calls
}

// ControlResult represents the result of executing a single control.
type ControlResult struct {
	ID           string               `json:"id" yaml:"id"`
	Name         string               `json:"name" yaml:"name"`
	Description  string               `json:"description,omitempty" yaml:"description,omitempty"`
	Severity     string               `json:"severity,omitempty" yaml:"severity,omitempty"`
	Tags         []string             `json:"tags,omitempty" yaml:"tags,omitempty"`
	Status       Status               `json:"status" yaml:"status"`
	Observations []ObservationResult  `json:"observations" yaml:"observations"`
	Message      string               `json:"message,omitempty" yaml:"message,omitempty"`
	Duration     time.Duration        `json:"duration_ms" yaml:"duration_ms"`
}

// ObservationResult represents the result of executing a single observation.
type ObservationResult struct {
	Plugin   string                 `json:"plugin" yaml:"plugin"`
	Config   map[string]interface{} `json:"config" yaml:"config"`
	Status   Status                 `json:"status" yaml:"status"`
	Evidence *wasm.Evidence         `json:"evidence,omitempty" yaml:"evidence,omitempty"`
	Error    *wasm.PluginError      `json:"error,omitempty" yaml:"error,omitempty"`
	Duration time.Duration          `json:"duration_ms" yaml:"duration_ms"`
}

// ResultSummary provides aggregate statistics about the execution.
type ResultSummary struct {
	TotalControls       int `json:"total_controls" yaml:"total_controls"`
	PassedControls      int `json:"passed_controls" yaml:"passed_controls"`
	FailedControls      int `json:"failed_controls" yaml:"failed_controls"`
	ErrorControls       int `json:"error_controls" yaml:"error_controls"`
	TotalObservations   int `json:"total_observations" yaml:"total_observations"`
	PassedObservations  int `json:"passed_observations" yaml:"passed_observations"`
	FailedObservations  int `json:"failed_observations" yaml:"failed_observations"`
	ErrorObservations   int `json:"error_observations" yaml:"error_observations"`
}

// NewExecutionResult creates a new execution result.
func NewExecutionResult(profileName, profileVersion string) *ExecutionResult {
	return &ExecutionResult{
		ProfileName:    profileName,
		ProfileVersion: profileVersion,
		StartTime:      time.Now(),
		Controls:       make([]ControlResult, 0),
	}
}

// AddControlResult adds a control result to the execution result.
// Thread-safe for concurrent calls during parallel execution.
func (r *ExecutionResult) AddControlResult(cr ControlResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Controls = append(r.Controls, cr)
}

// Finalize completes the execution result and calculates the summary.
func (r *ExecutionResult) Finalize() {
	r.EndTime = time.Now()
	r.Duration = r.EndTime.Sub(r.StartTime)
	r.calculateSummary()
}

// calculateSummary computes summary statistics from control results.
func (r *ExecutionResult) calculateSummary() {
	r.Summary = ResultSummary{
		TotalControls: len(r.Controls),
	}

	for _, ctrl := range r.Controls {
		// Count control statuses
		switch ctrl.Status {
		case StatusPass:
			r.Summary.PassedControls++
		case StatusFail:
			r.Summary.FailedControls++
		case StatusError:
			r.Summary.ErrorControls++
		}

		// Count observation statuses
		r.Summary.TotalObservations += len(ctrl.Observations)
		for _, obs := range ctrl.Observations {
			switch obs.Status {
			case StatusPass:
				r.Summary.PassedObservations++
			case StatusFail:
				r.Summary.FailedObservations++
			case StatusError:
				r.Summary.ErrorObservations++
			}
		}
	}
}
