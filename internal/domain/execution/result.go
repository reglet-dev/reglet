// Package execution provides domain models for execution results.
package execution

import (
	"sort"
	"sync"
	"time"

	"github.com/reglet-dev/reglet/internal/domain/constants"
	"github.com/reglet-dev/reglet/internal/domain/values"
)

// ExecutionResult represents the complete result of executing a profile.
//
//nolint:revive // ST1003: Name is intentional - "Result" alone lacks context in imports
type ExecutionResult struct {
	StartTime      time.Time
	EndTime        time.Time
	RegletVersion  string
	ProfileName    string
	ProfileVersion string
	Controls       []ControlResult
	Summary        ResultSummary
	Version        int
	Duration       time.Duration
	mu             sync.Mutex
	ExecutionID    values.ExecutionID
}

// ControlResult represents the result of executing a single control.
type ControlResult struct {
	ID                 string
	Name               string
	Description        string
	Severity           string
	Status             values.Status
	Message            string
	SkipReason         string
	Tags               []string
	ObservationResults []ObservationResult
	Index              int
	Duration           time.Duration
}

// ObservationResult represents the result of executing a single observation.
type ObservationResult struct {
	RawError     error
	Config       map[string]interface{}
	Evidence     *Evidence
	EvidenceMeta *EvidenceMeta
	Error        *PluginError
	Plugin       string
	Status       values.Status
	Expectations []ExpectationResult
	Duration     time.Duration
	// Loop fields - populated only when observation has a loop
	IsLoop    bool                // True if this is a loop parent observation
	LoopItem  interface{}         // The current item (for child observations)
	LoopIndex int                 // Index of this item in the loop
	Children  []ObservationResult // Child observation results (for parent)
}

// ExpectationResult represents the result of evaluating a single expectation expression.
// The Message field provides human-readable context about failures, constructed by the
// StatusAggregator which has full access to the evidence and expression evaluation context.
type ExpectationResult struct {
	Expression string
	Message    string
	Passed     bool
}

// ResultSummary provides aggregate statistics about the execution.
type ResultSummary struct {
	TotalControls      int
	PassedControls     int
	FailedControls     int
	ErrorControls      int
	SkippedControls    int
	TotalObservations  int
	PassedObservations int
	FailedObservations int
	ErrorObservations  int
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
// Re-exported from constants.DefaultMaxEvidenceSize for backward compatibility.
const DefaultMaxEvidenceSize = constants.DefaultMaxEvidenceSize

// EvidenceMeta contains metadata about evidence truncation.
type EvidenceMeta struct {
	Reason       string
	OriginalSize int
	TruncatedAt  int
	Truncated    bool
}
