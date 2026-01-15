package output

import (
	"time"

	"github.com/reglet-dev/reglet/internal/domain/execution"
	"github.com/reglet-dev/reglet/internal/domain/values"
)

// ExecutionResult represents the serialization format for execution results.
// This is the wire format for JSON/YAML output.
type ExecutionResult struct {
	StartTime      time.Time          `json:"start_time" yaml:"start_time"`
	EndTime        time.Time          `json:"end_time" yaml:"end_time"`
	RegletVersion  string             `json:"reglet_version,omitempty" yaml:"reglet_version,omitempty"`
	ProfileName    string             `json:"profile_name" yaml:"profile_name"`
	ProfileVersion string             `json:"profile_version" yaml:"profile_version"`
	Controls       []ControlResult    `json:"controls" yaml:"controls"`
	Summary        Summary            `json:"summary" yaml:"summary"`
	Version        int                `json:"version" yaml:"version"`
	Duration       time.Duration      `json:"duration_ms" yaml:"duration_ms"`
	ExecutionID    values.ExecutionID `json:"execution_id" yaml:"execution_id"`
}

// ControlResult represents the serialization format for a control result.
type ControlResult struct {
	ID           string              `json:"id" yaml:"id"`
	Name         string              `json:"name" yaml:"name"`
	Description  string              `json:"description,omitempty" yaml:"description,omitempty"`
	Severity     string              `json:"severity,omitempty" yaml:"severity,omitempty"`
	Status       values.Status       `json:"status" yaml:"status"`
	Message      string              `json:"message,omitempty" yaml:"message,omitempty"`
	SkipReason   string              `json:"skip_reason,omitempty" yaml:"skip_reason,omitempty"`
	Tags         []string            `json:"tags,omitempty" yaml:"tags,omitempty"`
	Observations []ObservationResult `json:"observations" yaml:"observations"`
	Index        int                 `json:"index" yaml:"index"`
	Duration     time.Duration       `json:"duration_ms" yaml:"duration_ms"`
}

// ObservationResult represents the serialization format for an observation result.
type ObservationResult struct {
	Config       map[string]any      `json:"config" yaml:"config"`
	Evidence     *execution.Evidence `json:"evidence,omitempty" yaml:"evidence,omitempty"`
	EvidenceMeta *EvidenceMeta       `json:"evidence_meta,omitempty" yaml:"evidence_meta,omitempty"`
	Error        *PluginError        `json:"error,omitempty" yaml:"error,omitempty"`
	Plugin       string              `json:"plugin" yaml:"plugin"`
	Status       values.Status       `json:"status" yaml:"status"`
	Expectations []ExpectationResult `json:"expectations,omitempty" yaml:"expectations,omitempty"`
	Duration     time.Duration       `json:"duration_ms" yaml:"duration_ms"`
}

// ExpectationResult represents the serialization format for an expectation result.
type ExpectationResult struct {
	Expression string `json:"expression" yaml:"expression"`
	Message    string `json:"message,omitempty" yaml:"message,omitempty"`
	Passed     bool   `json:"passed" yaml:"passed"`
}

// Summary represents the serialization format for execution summary.
type Summary struct {
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

// EvidenceMeta represents the serialization format for evidence metadata.
type EvidenceMeta struct {
	Reason       string `json:"reason,omitempty" yaml:"reason,omitempty"`
	OriginalSize int    `json:"original_size_bytes" yaml:"original_size_bytes"`
	TruncatedAt  int    `json:"truncated_at_bytes" yaml:"truncated_at_bytes"`
	Truncated    bool   `json:"truncated" yaml:"truncated"`
}

// PluginError represents the serialization format for plugin errors.
type PluginError struct {
	Code    string `json:"code" yaml:"code"`
	Message string `json:"message" yaml:"message"`
}

// FromDomain converts a domain ExecutionResult to an output DTO.
func FromDomain(r *execution.ExecutionResult) *ExecutionResult {
	if r == nil {
		return nil
	}

	controls := make([]ControlResult, len(r.Controls))
	for i, ctrl := range r.Controls {
		controls[i] = controlFromDomain(ctrl)
	}

	return &ExecutionResult{
		StartTime:      r.StartTime,
		EndTime:        r.EndTime,
		RegletVersion:  r.RegletVersion,
		ProfileName:    r.ProfileName,
		ProfileVersion: r.ProfileVersion,
		Controls:       controls,
		Summary:        summaryFromDomain(r.Summary),
		Version:        r.Version,
		Duration:       r.Duration,
		ExecutionID:    r.ExecutionID,
	}
}

func controlFromDomain(c execution.ControlResult) ControlResult {
	observations := make([]ObservationResult, len(c.ObservationResults))
	for i, obs := range c.ObservationResults {
		observations[i] = observationFromDomain(obs)
	}

	return ControlResult{
		ID:           c.ID,
		Name:         c.Name,
		Description:  c.Description,
		Severity:     c.Severity,
		Status:       c.Status,
		Message:      c.Message,
		SkipReason:   c.SkipReason,
		Tags:         c.Tags,
		Observations: observations,
		Index:        c.Index,
		Duration:     c.Duration,
	}
}

func observationFromDomain(o execution.ObservationResult) ObservationResult {
	expectations := make([]ExpectationResult, len(o.Expectations))
	for i, exp := range o.Expectations {
		expectations[i] = ExpectationResult{
			Expression: exp.Expression,
			Message:    exp.Message,
			Passed:     exp.Passed,
		}
	}

	var evidenceMeta *EvidenceMeta
	if o.EvidenceMeta != nil {
		evidenceMeta = &EvidenceMeta{
			Reason:       o.EvidenceMeta.Reason,
			OriginalSize: o.EvidenceMeta.OriginalSize,
			TruncatedAt:  o.EvidenceMeta.TruncatedAt,
			Truncated:    o.EvidenceMeta.Truncated,
		}
	}

	var pluginError *PluginError
	if o.Error != nil {
		pluginError = &PluginError{
			Code:    o.Error.Code,
			Message: o.Error.Message,
		}
	}

	return ObservationResult{
		Config:       o.Config,
		Evidence:     o.Evidence,
		EvidenceMeta: evidenceMeta,
		Error:        pluginError,
		Plugin:       o.Plugin,
		Status:       o.Status,
		Expectations: expectations,
		Duration:     o.Duration,
	}
}

func summaryFromDomain(s execution.ResultSummary) Summary {
	return Summary{
		TotalControls:      s.TotalControls,
		PassedControls:     s.PassedControls,
		FailedControls:     s.FailedControls,
		ErrorControls:      s.ErrorControls,
		SkippedControls:    s.SkippedControls,
		TotalObservations:  s.TotalObservations,
		PassedObservations: s.PassedObservations,
		FailedObservations: s.FailedObservations,
		ErrorObservations:  s.ErrorObservations,
	}
}
