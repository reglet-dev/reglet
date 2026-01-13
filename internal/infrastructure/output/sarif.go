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
//	formatter := output.NewSARIFFormatter(os.Stdout, WithProfilePath("profile.yaml"))
//	if err := formatter.Format(result); err != nil {
//	    log.Fatal(err)
//	}
type SARIFFormatter struct {
	writer      io.Writer
	profilePath string
}

// SARIFOption configures the SARIF formatter.
type SARIFOption func(*SARIFFormatter)

// NewSARIFFormatter creates a new SARIF formatter.
func NewSARIFFormatter(w io.Writer, opts ...SARIFOption) *SARIFFormatter {
	f := &SARIFFormatter{writer: w}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// WithProfilePath sets the profile path for SARIF location resolution.
func WithProfilePath(path string) SARIFOption {
	return func(f *SARIFFormatter) {
		f.profilePath = path
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
