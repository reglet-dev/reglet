// Package execution provides domain models for execution results.
package execution

import (
	"sort"
	"sync"
	"time"

	"github.com/reglet-dev/reglet/internal/domain/values"
)

// ExecutionResult represents the complete result of executing a profile.
//
//nolint:revive // ST1003: Name is intentional - "Result" alone lacks context in imports
type ExecutionResult struct {
	StartTime      time.Time       `json:"start_time" yaml:"start_time"`
	EndTime        time.Time       `json:"end_time" yaml:"end_time"`
	RegletVersion  string          `json:"reglet_version,omitempty" yaml:"reglet_version,omitempty"`
	ProfileName    string          `json:"profile_name" yaml:"profile_name"`
	ProfileVersion string          `json:"profile_version" yaml:"profile_version"`
	Controls       []ControlResult `json:"controls" yaml:"controls"`
	Summary        ResultSummary   `json:"summary" yaml:"summary"`
	Version        int             `json:"version" yaml:"version"`
	Duration       time.Duration   `json:"duration_ms" yaml:"duration_ms"`
	mu             sync.Mutex
	ExecutionID    values.ExecutionID `json:"execution_id" yaml:"execution_id"`
}

// ControlResult represents the result of executing a single control.
type ControlResult struct {
	ID                 string              `json:"id" yaml:"id"`
	Name               string              `json:"name" yaml:"name"`
	Description        string              `json:"description,omitempty" yaml:"description,omitempty"`
	Severity           string              `json:"severity,omitempty" yaml:"severity,omitempty"`
	Status             values.Status       `json:"status" yaml:"status"`
	Message            string              `json:"message,omitempty" yaml:"message,omitempty"`
	SkipReason         string              `json:"skip_reason,omitempty" yaml:"skip_reason,omitempty"`
	Tags               []string            `json:"tags,omitempty" yaml:"tags,omitempty"`
	ObservationResults []ObservationResult `json:"observations" yaml:"observations"`
	Index              int                 `json:"index" yaml:"index"`
	Duration           time.Duration       `json:"duration_ms" yaml:"duration_ms"`
}

// ObservationResult represents the result of executing a single observation.
type ObservationResult struct {
	RawError     error                  `json:"-" yaml:"-"`
	Config       map[string]interface{} `json:"config" yaml:"config"`
	Evidence     *Evidence              `json:"evidence,omitempty" yaml:"evidence,omitempty"`
	EvidenceMeta *EvidenceMeta          `json:"evidence_meta,omitempty" yaml:"evidence_meta,omitempty"`
	Error        *PluginError           `json:"error,omitempty" yaml:"error,omitempty"`
	Plugin       string                 `json:"plugin" yaml:"plugin"`
	Status       values.Status          `json:"status" yaml:"status"`
	Expectations []ExpectationResult    `json:"expectations,omitempty" yaml:"expectations,omitempty"`
	Duration     time.Duration          `json:"duration_ms" yaml:"duration_ms"`
}

// ExpectationResult represents the result of evaluating a single expectation expression.
// The Message field provides human-readable context about failures, constructed by the
// StatusAggregator which has full access to the evidence and expression evaluation context.
type ExpectationResult struct {
	Expression string `json:"expression" yaml:"expression"`
	Message    string `json:"message,omitempty" yaml:"message,omitempty"`
	Passed     bool   `json:"passed" yaml:"passed"`
}

// ResultSummary provides aggregate statistics about the execution.
type ResultSummary struct {
	TotalControls      int `json:"total_controls" yaml:"total_controls"`
	PassedControls     int `json:"passed_controls" yaml:"passed_controls"`
	FailedControls     int `json:"failed_controls" yaml:"failed_controls"`
	ErrorControls      int `json:"error_controls" yaml:"error_controls"`
	SkippedControls    int `json:"skipped_controls" yaml:"skipped_controls"`
	TotalObservations  int `json:"total_observations" yaml:"total_observations"`
	PassedObservations int `json:"passed_observations" yaml:"passed_observations"`
	FailedObservations int `json:"failed_observations" yaml:"failed_observations"`
	ErrorObservations  int `json:"error_observations" yaml:"error_observations"`
}

// NewExecutionResult creates a new execution result.
func NewExecutionResult(profileName, profileVersion string) *ExecutionResult {
	return NewExecutionResultWithID(values.NewExecutionID(), profileName, profileVersion)
}

// NewExecutionResultWithID creates a new execution result with a specific ID.
func NewExecutionResultWithID(id values.ExecutionID, profileName, profileVersion string) *ExecutionResult {
	return &ExecutionResult{
		ExecutionID:    id,
		ProfileName:    profileName,
		ProfileVersion: profileVersion,
		StartTime:      time.Now(),
		Controls:       make([]ControlResult, 0),
		Version:        1,
	}
}

// GetID returns the execution ID.
func (r *ExecutionResult) GetID() values.ExecutionID {
	return r.ExecutionID
}

// GetVersion returns the optimistic locking version.
func (r *ExecutionResult) GetVersion() int {
	return r.Version
}

// IncrementVersion increments the version counter.
func (r *ExecutionResult) IncrementVersion() {
	r.Version++
}

// AddControlResult adds a control result to the execution result.
// Thread-safe for concurrent calls during parallel execution.
func (r *ExecutionResult) AddControlResult(cr ControlResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Controls = append(r.Controls, cr)
}

// AddPartialResult adds a control result from a partial execution (e.g. worker).
func (r *ExecutionResult) AddPartialResult(cr ControlResult) {
	r.AddControlResult(cr)
}

// GetControlStatus returns the status of a control by ID.
// Returns the status and a boolean indicating if the control was found.
// Thread-safe.
func (r *ExecutionResult) GetControlStatus(id string) (values.Status, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, ctrl := range r.Controls {
		if ctrl.ID == id {
			return ctrl.Status, true
		}
	}
	return "", false
}

// GetControlResultByID returns a pointer to the control result with the given ID, or nil if not found.
// Thread-safe.
func (r *ExecutionResult) GetControlResultByID(id string) *ControlResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.Controls {
		if r.Controls[i].ID == id {
			return &r.Controls[i]
		}
	}
	return nil
}

// IsComplete checks if the number of executed controls matches the expected count.
func (r *ExecutionResult) IsComplete(expectedControlCount int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.Controls) >= expectedControlCount
}

// Finalize completes the execution result and calculates the summary.
// Controls are sorted by their original definition order for deterministic output.
func (r *ExecutionResult) Finalize() {
	r.EndTime = time.Now()
	r.Duration = r.EndTime.Sub(r.StartTime)

	// Sort controls by original definition order for deterministic output
	sort.Slice(r.Controls, func(i, j int) bool {
		return r.Controls[i].Index < r.Controls[j].Index
	})

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
		case values.StatusPass:
			r.Summary.PassedControls++
		case values.StatusFail:
			r.Summary.FailedControls++
		case values.StatusError:
			r.Summary.ErrorControls++
		case values.StatusSkipped:
			r.Summary.SkippedControls++
		}

		// Count observation statuses
		r.Summary.TotalObservations += len(ctrl.ObservationResults)
		for _, obs := range ctrl.ObservationResults {
			switch obs.Status {
			case values.StatusPass:
				r.Summary.PassedObservations++
			case values.StatusFail:
				r.Summary.FailedObservations++
			case values.StatusError:
				r.Summary.ErrorObservations++
			}
		}
	}
}

// Evidence represents observation results (proof of compliance state).
// This is a core domain concept representing the evidence collected during a check.
type Evidence struct {
	Timestamp time.Time
	Error     *PluginError
	Data      map[string]interface{}
	Raw       *string
	Status    bool
}

// PluginError represents an error from plugin execution.
// This is a domain concept representing a failure in collecting evidence.
type PluginError struct {
	Code    string
	Message string
}

// Error implements the error interface
func (e *PluginError) Error() string {
	return e.Code + ": " + e.Message
}

// DefaultMaxEvidenceSize is the default limit for evidence size (1MB).
const DefaultMaxEvidenceSize = 1 * 1024 * 1024

// EvidenceMeta contains metadata about evidence truncation.
type EvidenceMeta struct {
	Truncated    bool   `json:"truncated" yaml:"truncated"`
	OriginalSize int    `json:"original_size_bytes" yaml:"original_size_bytes"`
	TruncatedAt  int    `json:"truncated_at_bytes" yaml:"truncated_at_bytes"`
	Reason       string `json:"reason,omitempty" yaml:"reason,omitempty"`
}
