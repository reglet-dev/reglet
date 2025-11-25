package output

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jrose/reglet/internal/engine"
)

// TableFormatter formats execution results as a human-readable table.
type TableFormatter struct {
	writer io.Writer
}

// NewTableFormatter creates a new table formatter.
func NewTableFormatter(w io.Writer) *TableFormatter {
	return &TableFormatter{writer: w}
}

// Format writes the execution result as a table.
func (f *TableFormatter) Format(result *engine.ExecutionResult) error {
	// Print header
	fmt.Fprintf(f.writer, "Profile: %s (v%s)\n", result.ProfileName, result.ProfileVersion)
	fmt.Fprintf(f.writer, "Executed: %s\n", result.StartTime.Format(time.RFC3339))
	fmt.Fprintf(f.writer, "Duration: %s\n", result.Duration.Round(time.Millisecond))
	fmt.Fprintln(f.writer)

	// Print controls table
	if len(result.Controls) == 0 {
		fmt.Fprintln(f.writer, "No controls executed.")
		return nil
	}

	fmt.Fprintln(f.writer, "Controls:")
	fmt.Fprintln(f.writer, strings.Repeat("─", 80))

	for _, ctrl := range result.Controls {
		f.formatControl(ctrl)
	}

	fmt.Fprintln(f.writer, strings.Repeat("─", 80))
	fmt.Fprintln(f.writer)

	// Print summary
	f.formatSummary(result.Summary)

	return nil
}

// formatControl formats a single control.
func (f *TableFormatter) formatControl(ctrl engine.ControlResult) {
	// Status symbol
	statusSymbol := f.getStatusSymbol(ctrl.Status)

	// Control header
	fmt.Fprintf(f.writer, "%s %s: %s\n", statusSymbol, ctrl.ID, ctrl.Name)

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
	fmt.Fprintf(f.writer, "  Status: %s\n", strings.ToUpper(string(ctrl.Status)))
	if ctrl.Message != "" {
		fmt.Fprintf(f.writer, "  Message: %s\n", ctrl.Message)
	}

	// Duration
	fmt.Fprintf(f.writer, "  Duration: %s\n", ctrl.Duration.Round(time.Millisecond))

	// Observations
	if len(ctrl.Observations) > 0 {
		fmt.Fprintln(f.writer, "  Observations:")
		for i, obs := range ctrl.Observations {
			f.formatObservation(obs, i+1)
		}
	}

	fmt.Fprintln(f.writer)
}

// formatObservation formats a single observation.
func (f *TableFormatter) formatObservation(obs engine.ObservationResult, index int) {
	statusSymbol := f.getStatusSymbol(obs.Status)
	fmt.Fprintf(f.writer, "    %d. %s Plugin: %s (%s)\n", index, statusSymbol, obs.Plugin, obs.Status)

	// Show error if present
	if obs.Error != nil {
		fmt.Fprintf(f.writer, "       Error: [%s] %s\n", obs.Error.Code, obs.Error.Message)
	}

	// Show evidence summary if present
	if obs.Evidence != nil {
		fmt.Fprintf(f.writer, "       Evidence: collected at %s\n", obs.Evidence.Timestamp.Format(time.RFC3339))
		if len(obs.Evidence.Data) > 0 {
			// Show evidence data fields
			for key, value := range obs.Evidence.Data {
				fmt.Fprintf(f.writer, "       - %s: %v\n", key, value)
			}
		}
	}

	fmt.Fprintf(f.writer, "       Duration: %s\n", obs.Duration.Round(time.Millisecond))
}

// formatSummary formats the summary statistics.
func (f *TableFormatter) formatSummary(summary engine.ResultSummary) {
	fmt.Fprintln(f.writer, "Summary:")
	fmt.Fprintln(f.writer, strings.Repeat("─", 80))

	// Controls summary
	fmt.Fprintf(f.writer, "Controls:     %d total\n", summary.TotalControls)
	fmt.Fprintf(f.writer, "  ✓ Passed:   %d\n", summary.PassedControls)
	fmt.Fprintf(f.writer, "  ✗ Failed:   %d\n", summary.FailedControls)
	fmt.Fprintf(f.writer, "  ⚠ Errors:   %d\n", summary.ErrorControls)
	fmt.Fprintln(f.writer)

	// Observations summary
	fmt.Fprintf(f.writer, "Observations: %d total\n", summary.TotalObservations)
	fmt.Fprintf(f.writer, "  ✓ Passed:   %d\n", summary.PassedObservations)
	fmt.Fprintf(f.writer, "  ✗ Failed:   %d\n", summary.FailedObservations)
	fmt.Fprintf(f.writer, "  ⚠ Errors:   %d\n", summary.ErrorObservations)

	fmt.Fprintln(f.writer, strings.Repeat("─", 80))
}

// getStatusSymbol returns a symbol for the given status.
func (f *TableFormatter) getStatusSymbol(status engine.Status) string {
	switch status {
	case engine.StatusPass:
		return "✓"
	case engine.StatusFail:
		return "✗"
	case engine.StatusError:
		return "⚠"
	default:
		return "?"
	}
}
