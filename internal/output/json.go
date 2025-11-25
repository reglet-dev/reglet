package output

import (
	"encoding/json"
	"io"

	"github.com/jrose/reglet/internal/engine"
)

// JSONFormatter formats execution results as JSON.
type JSONFormatter struct {
	writer io.Writer
	indent bool
}

// NewJSONFormatter creates a new JSON formatter.
// If indent is true, the output will be pretty-printed with indentation.
func NewJSONFormatter(w io.Writer, indent bool) *JSONFormatter {
	return &JSONFormatter{
		writer: w,
		indent: indent,
	}
}

// Format writes the execution result as JSON.
func (f *JSONFormatter) Format(result *engine.ExecutionResult) error {
	var data []byte
	var err error

	if f.indent {
		data, err = json.MarshalIndent(result, "", "  ")
	} else {
		data, err = json.Marshal(result)
	}

	if err != nil {
		return err
	}

	_, err = f.writer.Write(data)
	if err != nil {
		return err
	}

	// Add newline for better terminal output
	_, err = f.writer.Write([]byte("\n"))
	return err
}
