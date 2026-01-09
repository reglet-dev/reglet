// Package output provides formatters for Reglet execution results.
package output

import (
	"fmt"
	"io"

	"github.com/owenrumney/go-sarif/v3/pkg/report/v210/sarif"
	"github.com/reglet-dev/reglet/internal/domain/execution"
)

// SARIFFormatter formats execution results as SARIF 2.1.0 JSON.
// It maps Reglet controls to SARIF rules and observations to results with locations.
//
// Usage:
//
//	formatter := output.NewSARIFFormatter(os.Stdout, "profile.yaml")
//	if err := formatter.Format(result); err != nil {
//	    log.Fatal(err)
//	}
type SARIFFormatter struct {
	writer      io.Writer
	profilePath string
}

// NewSARIFFormatter creates a new SARIF formatter.
// profilePath is used to resolve relative paths for file locations.
func NewSARIFFormatter(writer io.Writer, profilePath string) *SARIFFormatter {
	return &SARIFFormatter{
		writer:      writer,
		profilePath: profilePath,
	}
}

// Format writes the execution result as SARIF 2.1.0 JSON.
// Returns error if SARIF creation or marshaling fails.
func (f *SARIFFormatter) Format(result *execution.ExecutionResult) error {
	// 1. Create SARIF report
	report := sarif.NewReport()

	// 2. Create run with tool info
	run := sarif.NewRunWithInformationURI("Reglet", "https://reglet.dev")
	run.Tool.Driver.Version = &result.RegletVersion
	run.Tool.Driver.Organization = ptrString("Reglet")

	// 3. Map execution result to run
	mapper := newSARIFMapper(result, f.profilePath)
	mapper.mapToRun(run)

	// 4. Add run to report
	report.AddRun(run)

	// 5. Write to output
	// Sarif library handles marshaling and writing
	if err := report.Write(f.writer); err != nil {
		return fmt.Errorf("failed to write SARIF output: %w", err)
	}

	// 6. Add newline for terminal output (already handled by Write or not? Write does not add newline usually)
	_, err := f.writer.Write([]byte("\n"))
	return err
}

func ptrString(s string) *string {
	return &s
}
