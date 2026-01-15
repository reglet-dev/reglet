package output

import (
	"io"

	"github.com/goccy/go-yaml"
	"github.com/reglet-dev/reglet/internal/domain/execution"
)

// YAMLFormatter formats execution results as YAML.
type YAMLFormatter struct {
	writer io.Writer
	indent int
}

// YAMLOption configures the YAML formatter.
type YAMLOption func(*YAMLFormatter)

// WithYAMLIndent sets the indentation level for YAML output.
func WithYAMLIndent(indent int) YAMLOption {
	return func(f *YAMLFormatter) {
		f.indent = indent
	}
}

// NewYAMLFormatter creates a new YAML formatter.
func NewYAMLFormatter(w io.Writer, opts ...YAMLOption) *YAMLFormatter {
	f := &YAMLFormatter{
		writer: w,
		indent: 2,
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// Format writes the execution result as YAML.
func (f *YAMLFormatter) Format(result *execution.ExecutionResult) error {
	// Convert domain entity to output representation
	out := FromDomain(result)

	encoder := yaml.NewEncoder(f.writer, yaml.Indent(f.indent))

	encodeErr := encoder.Encode(out)
	closeErr := encoder.Close()

	if encodeErr != nil {
		return encodeErr
	}

	return closeErr
}
