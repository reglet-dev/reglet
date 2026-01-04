package output

import (
	"encoding/xml"
	"fmt"
	"io"

	"github.com/whiskeyjimbo/reglet/internal/domain/execution"
	"github.com/whiskeyjimbo/reglet/internal/domain/values"
)

// JUnitFormatter formats execution results as JUnit XML.
type JUnitFormatter struct {
	writer io.Writer
}

// NewJUnitFormatter creates a new JUnit formatter.
func NewJUnitFormatter(w io.Writer) *JUnitFormatter {
	return &JUnitFormatter{
		writer: w,
	}
}

// JUnitTestSuites JUnit XML structures
type JUnitTestSuites struct {
	XMLName    xml.Name         `xml:"testsuites"`
	Name       string           `xml:"name,attr"`
	Tests      int              `xml:"tests,attr"`
	Failures   int              `xml:"failures,attr"`
	Errors     int              `xml:"errors,attr"`
	Time       float64          `xml:"time,attr"`
	TestSuites []JUnitTestSuite `xml:"testsuite"`
}

type JUnitTestSuite struct {
	XMLName   xml.Name        `xml:"testsuite"`
	Name      string          `xml:"name,attr"`
	Tests     int             `xml:"tests,attr"`
	Failures  int             `xml:"failures,attr"`
	Errors    int             `xml:"errors,attr"`
	Skipped   int             `xml:"skipped,attr"`
	Time      float64         `xml:"time,attr"`
	TestCases []JUnitTestCase `xml:"testcase"`
}

type JUnitTestCase struct {
	XMLName   xml.Name      `xml:"testcase"`
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Time      float64       `xml:"time,attr"`
	Failure   *JUnitFailure `xml:"failure,omitempty"`
	Error     *JUnitError   `xml:"error,omitempty"`
	Skipped   *JUnitSkipped `xml:"skipped,omitempty"`
}

type JUnitFailure struct {
	Message string `xml:"message,attr"`
	Content string `xml:",chardata"`
}

type JUnitError struct {
	Message string `xml:"message,attr"`
	Content string `xml:",chardata"`
}

type JUnitSkipped struct {
	Message string `xml:"message,attr,omitempty"`
}

// Format writes the execution result as JUnit XML.
func (f *JUnitFormatter) Format(result *execution.ExecutionResult) error {
	// Create a single test suite for the profile execution
	suite := JUnitTestSuite{
		Name:     result.ProfileName,
		Tests:    result.Summary.TotalControls,
		Failures: result.Summary.FailedControls,
		Errors:   result.Summary.ErrorControls,
		Skipped:  result.Summary.SkippedControls,
		Time:     result.Duration.Seconds(),
	}

	for _, ctrl := range result.Controls {
		c := JUnitTestCase{
			Name:      ctrl.ID,   // Control ID as test name
			ClassName: ctrl.Name, // Control Name as classname
			Time:      ctrl.Duration.Seconds(),
		}

		switch ctrl.Status {
		case values.StatusFail:
			c.Failure = &JUnitFailure{
				Message: ctrl.Message,
				Content: formatObservations(ctrl),
			}
		case values.StatusError:
			c.Error = &JUnitError{
				Message: ctrl.Message,
				Content: formatObservations(ctrl),
			}
		case values.StatusSkipped:
			c.Skipped = &JUnitSkipped{
				Message: ctrl.SkipReason,
			}
		}

		suite.TestCases = append(suite.TestCases, c)
	}

	suites := JUnitTestSuites{
		Name:       "Reglet Execution",
		Tests:      result.Summary.TotalControls,
		Failures:   result.Summary.FailedControls,
		Errors:     result.Summary.ErrorControls,
		Time:       result.Duration.Seconds(),
		TestSuites: []JUnitTestSuite{suite},
	}

	_, err := f.writer.Write([]byte(xml.Header))
	if err != nil {
		return err
	}

	encoder := xml.NewEncoder(f.writer)
	encoder.Indent("", "  ")
	if err := encoder.Encode(suites); err != nil {
		return err
	}

	_, err = f.writer.Write([]byte("\n"))
	return err
}

func formatObservations(ctrl execution.ControlResult) string {
	var out string
	for _, obs := range ctrl.ObservationResults {
		if obs.Status != values.StatusPass {
			out += fmt.Sprintf("Observation (%s): %s\n", obs.Plugin, obs.Status)
			if obs.Error != nil {
				out += fmt.Sprintf("Error: %s\n", obs.Error.Message)
			}
			if obs.Evidence != nil {
				out += fmt.Sprintf("Evidence: %v\n", obs.Evidence.Data)
			}
			out += "\n"
		}
	}
	return out
}
