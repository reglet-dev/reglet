// Package output holds various output options
package output

import (
	"encoding/json"
	"io"

	"github.com/reglet-dev/reglet/internal/domain/execution"
)

// JSONFormatter formats execution results as JSON.
type JSONFormatter struct {
	writer io.Writer
	config baseOutputConfig
}

// JSONOption configures the JSON formatter.
type JSONOption func(*JSONFormatter)

// NewJSONFormatter creates a new JSON formatter.
// If no options are provided, defaults to indented output.
func NewJSONFormatter(w io.Writer, opts ...JSONOption) *JSONFormatter {
	f := &JSONFormatter{
		writer: w,
		config: baseOutputConfig{indent: true}, // Default: indented
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// WithJSONIndent sets indentation for JSON output.
func WithJSONIndent(enabled bool) JSONOption {
	return func(f *JSONFormatter) {
		f.config.indent = enabled
	}
}

// Format writes the execution result as JSON.
func (f *JSONFormatter) Format(result *execution.ExecutionResult) error {
	// Convert domain entity to output representation
	out := FromDomain(result)

	var data []byte
	var err error

	if f.config.indent {
		data, err = json.MarshalIndent(out, "", "  ")
	} else {
		data, err = json.Marshal(out)
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
