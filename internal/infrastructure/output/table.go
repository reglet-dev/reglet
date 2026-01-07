package output

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/whiskeyjimbo/reglet/internal/domain/execution"
	"github.com/whiskeyjimbo/reglet/internal/domain/values"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorGray   = "\033[90m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

// TableFormatter formats execution results as a human-readable table.
type TableFormatter struct {
	writer      io.Writer
	EnableColor bool
}

// NewTableFormatter creates a new table formatter.
func NewTableFormatter(w io.Writer) *TableFormatter {
	return &TableFormatter{
		writer:      w,
		EnableColor: true, // Default to true, caller can disable
	}
}

// colorize returns the string wrapped in ANSI color codes if enabled.
func (f *TableFormatter) colorize(text, code string) string {
	if !f.EnableColor {
		return text
	}
	return code + text + colorReset
}

// Format writes the execution result as a table.
//
//nolint:errcheck // Table formatting errors are non-critical (best-effort terminal output)
func (f *TableFormatter) Format(result *execution.ExecutionResult) error {
	// Print header
	fmt.Fprintln(f.writer, f.colorize(strings.Repeat("─", 80), colorGray))
	fmt.Fprintf(f.writer, "Profile: %s (v%s)\n", f.colorize(result.ProfileName, colorBold), result.ProfileVersion)
	fmt.Fprintf(f.writer, "Executed: %s\n", result.StartTime.Format(time.RFC3339))
	fmt.Fprintf(f.writer, "Duration: %s\n", result.Duration.Round(time.Millisecond))
	fmt.Fprintln(f.writer)

	// Print controls table
	if len(result.Controls) == 0 {
		fmt.Fprintln(f.writer, "No controls executed.")
		return nil
	}

	fmt.Fprintln(f.writer, f.colorize("Controls:", colorBold))
	fmt.Fprintln(f.writer, f.colorize(strings.Repeat("─", 80), colorGray))

	for _, ctrl := range result.Controls {
		f.formatControl(ctrl)
	}

	fmt.Fprintln(f.writer, f.colorize(strings.Repeat("─", 80), colorGray))
	fmt.Fprintln(f.writer)

	// Print summary
	f.formatSummary(result.Summary)

	return nil
}

// formatControl formats a single control.
//
//nolint:errcheck // Best-effort terminal output
func (f *TableFormatter) formatControl(ctrl execution.ControlResult) {
	// Status symbol and color
	statusSymbol, statusColor := f.getStatusInfo(ctrl.Status)
	coloredSymbol := f.colorize(statusSymbol, statusColor)
	coloredID := f.colorize(ctrl.ID, statusColor)

	// Control header
	fmt.Fprintf(f.writer, "%s %s: %s\n", coloredSymbol, coloredID, ctrl.Name)

	// Description
	if ctrl.Description != "" {
		fmt.Fprintf(f.writer, "  Description: %s\n", ctrl.Description)
	}

	// Severity
	if ctrl.Severity != "" {
		fmt.Fprintf(f.writer, "  Severity: %s\n", ctrl.Severity)
	}

	// Tags
	if len(ctrl.Tags) > 0 {
		fmt.Fprintf(f.writer, "  Tags: %s\n", strings.Join(ctrl.Tags, ", "))
	}

	// Status and message
	statusText := f.colorize(strings.ToUpper(string(ctrl.Status)), statusColor)
	fmt.Fprintf(f.writer, "  Status: %s\n", statusText)
	if ctrl.Message != "" {
		fmt.Fprintf(f.writer, "  Message: %s\n", ctrl.Message)
	}

	// Explicit skip reason if different from message or for clarity
	if ctrl.SkipReason != "" && ctrl.SkipReason != ctrl.Message {
		fmt.Fprintf(f.writer, "  Skip Reason: %s\n", ctrl.SkipReason)
	}

	// Duration
	fmt.Fprintf(f.writer, "  Duration: %s\n", ctrl.Duration.Round(time.Millisecond))

	// Observations
	if len(ctrl.ObservationResults) > 0 {
		fmt.Fprintln(f.writer, "  Observations:")
		for i, obs := range ctrl.ObservationResults {
			f.formatObservation(obs, i+1)
		}
	}

	fmt.Fprintln(f.writer)
}

// formatObservation formats a single observation.
//
//nolint:errcheck // Best-effort terminal output
func (f *TableFormatter) formatObservation(obs execution.ObservationResult, index int) {
	statusSymbol, statusColor := f.getStatusInfo(obs.Status)
	coloredSymbol := f.colorize(statusSymbol, statusColor)
	pluginName := f.colorize(obs.Plugin, colorCyan)

	fmt.Fprintf(f.writer, "    %d. %s Plugin: %s (%s)\n", index, coloredSymbol, pluginName, obs.Status)

	f.formatObsError(obs)
	f.formatFailedExpectations(obs)
	f.formatEvidence(obs)

	fmt.Fprintf(f.writer, "       Duration: %s\n", obs.Duration.Round(time.Millisecond))
}

// formatObsError formats the error section of an observation.
//
//nolint:errcheck // Best-effort terminal output
func (f *TableFormatter) formatObsError(obs execution.ObservationResult) {
	if obs.Error == nil {
		return
	}
	errMsg := fmt.Sprintf("[%s] %s", obs.Error.Code, obs.Error.Message)
	fmt.Fprintf(f.writer, "       %s: %s\n", f.colorize("Error", colorRed), errMsg)
}

// formatFailedExpectations formats the failed expectations section.
//
//nolint:errcheck // Best-effort terminal output
func (f *TableFormatter) formatFailedExpectations(obs execution.ObservationResult) {
	if len(obs.Expectations) == 0 {
		return
	}

	var failedExpectations []execution.ExpectationResult
	for _, exp := range obs.Expectations {
		if !exp.Passed {
			failedExpectations = append(failedExpectations, exp)
		}
	}

	if len(failedExpectations) == 0 {
		return
	}

	fmt.Fprintf(f.writer, "       %s:\n", f.colorize("Failed Expectations", colorRed))
	for _, exp := range failedExpectations {
		fmt.Fprintf(f.writer, "         - %s\n", exp.Expression)
		if exp.Message != "" {
			fmt.Fprintf(f.writer, "           %s\n", f.colorize(exp.Message, colorYellow))
		}
	}
}

// formatEvidence formats the evidence section of an observation.
//
//nolint:errcheck // Best-effort terminal output
func (f *TableFormatter) formatEvidence(obs execution.ObservationResult) {
	if obs.Evidence == nil {
		return
	}

	keys := f.collectEvidenceKeys(obs.Evidence.Data)
	if len(keys) == 0 {
		return
	}

	fmt.Fprintf(f.writer, "       Evidence:\n")
	for _, key := range keys {
		f.formatEvidenceValue(key, obs.Evidence.Data[key])
	}
}

// collectEvidenceKeys collects valid keys for evidence display.
func (f *TableFormatter) collectEvidenceKeys(data map[string]interface{}) []string {
	var keys []string
	for k, v := range data {
		// Skip internal/redundant keys
		if k == "status" || k == "error" || k == "success" {
			continue
		}
		// Skip empty/nil values
		if v == nil {
			continue
		}
		if s, ok := v.(string); ok && s == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// formatEvidenceValue formats a single evidence key-value pair.
//
//nolint:errcheck // Best-effort terminal output
func (f *TableFormatter) formatEvidenceValue(key string, value interface{}) {
	valStr := f.formatValue(value)

	if strings.Contains(valStr, "\n") {
		fmt.Fprintf(f.writer, "         - %s:\n", f.colorize(key, colorBlue))
		for _, line := range strings.Split(valStr, "\n") {
			fmt.Fprintf(f.writer, "           %s\n", line)
		}
	} else {
		fmt.Fprintf(f.writer, "         - %s: %s\n", f.colorize(key, colorBlue), valStr)
	}
}

// formatValue formats a value for display
func (f *TableFormatter) formatValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case map[string]interface{}:
		return f.formatErrorDetail(v, "")
	case []interface{}:
		// Check if it's a list of maps (like mx_records)
		if len(v) > 0 {
			if _, ok := v[0].(map[string]interface{}); ok {
				var lines []string
				for _, item := range v {
					if m, ok := item.(map[string]interface{}); ok {
						// Format map as key=value pairs
						var parts []string
						// Sort keys for deterministic output
						var keys []string
						for k := range m {
							keys = append(keys, k)
						}
						sort.Strings(keys)
						for _, k := range keys {
							parts = append(parts, fmt.Sprintf("%s=%v", k, m[k]))
						}
						lines = append(lines, fmt.Sprintf("{%s}", strings.Join(parts, ", ")))
					} else {
						lines = append(lines, fmt.Sprintf("%v", item))
					}
				}
				// Indent the list items
				return "\n           " + strings.Join(lines, "\n           ")
			}
		}
		return fmt.Sprintf("%v", v)
	default:
		return fmt.Sprintf("%v", value)
	}
}

// formatSummary formats the summary statistics.
//
//nolint:errcheck // Best-effort terminal output
func (f *TableFormatter) formatSummary(summary execution.ResultSummary) {
	fmt.Fprintln(f.writer, f.colorize("Summary:", colorBold))
	fmt.Fprintln(f.writer, f.colorize(strings.Repeat("─", 80), colorGray))

	// Controls summary
	fmt.Fprintf(f.writer, "Controls:     %d total\n", summary.TotalControls)
	fmt.Fprintf(f.writer, "  %s Passed:   %d\n", f.colorize("✓", colorGreen), summary.PassedControls)
	fmt.Fprintf(f.writer, "  %s Failed:   %d\n", f.colorize("✗", colorRed), summary.FailedControls)
	fmt.Fprintf(f.writer, "  %s Errors:   %d\n", f.colorize("⚠", colorYellow), summary.ErrorControls)
	fmt.Fprintf(f.writer, "  %s Skipped:  %d\n", f.colorize("⊘", colorGray), summary.SkippedControls)
	fmt.Fprintln(f.writer)

	// Observations summary
	fmt.Fprintf(f.writer, "Observations: %d total\n", summary.TotalObservations)
	fmt.Fprintf(f.writer, "  %s Passed:   %d\n", f.colorize("✓", colorGreen), summary.PassedObservations)
	fmt.Fprintf(f.writer, "  %s Failed:   %d\n", f.colorize("✗", colorRed), summary.FailedObservations)
	fmt.Fprintf(f.writer, "  %s Errors:   %d\n", f.colorize("⚠", colorYellow), summary.ErrorObservations)

	fmt.Fprintln(f.writer, f.colorize(strings.Repeat("─", 80), colorGray))
}

// formatErrorDetail formats a structured error with type, code, message, and wrapped errors.
func (f *TableFormatter) formatErrorDetail(errMap map[string]interface{}, indent string) string {
	var parts []string

	// Extract error type
	if errType, ok := errMap["type"].(string); ok && errType != "" && errType != "internal" {
		parts = append(parts, fmt.Sprintf("[%s]", errType))
	}

	// Extract error code
	if code, ok := errMap["code"].(string); ok && code != "" {
		parts = append(parts, fmt.Sprintf("(%s)", code))
	}

	// Extract message
	if message, ok := errMap["message"].(string); ok {
		parts = append(parts, message)
	}

	result := indent + strings.Join(parts, " ")

	// Handle wrapped errors
	if wrapped, ok := errMap["wrapped"].(map[string]interface{}); ok {
		result += "\n" + f.formatErrorDetail(wrapped, indent+"  caused by: ")
	}

	return result
}

// getStatusInfo returns a symbol and color for the given status.
func (f *TableFormatter) getStatusInfo(status values.Status) (string, string) {
	switch status {
	case values.StatusPass:
		return "✓", colorGreen
	case values.StatusFail:
		return "✗", colorRed
	case values.StatusError:
		return "⚠", colorYellow
	case values.StatusSkipped:
		return "⊘", colorGray
	default:
		return "?", colorReset
	}
}

// getStatusSymbol returns a symbol for the given status (legacy helper)
func (f *TableFormatter) getStatusSymbol(status values.Status) string {
	s, _ := f.getStatusInfo(status)
	return s
}
