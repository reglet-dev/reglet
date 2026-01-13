package output

import (
	"io"

	"github.com/goccy/go-yaml"
	"github.com/reglet-dev/reglet/internal/domain/execution"
)

// YAMLFormatter formats execution results as YAML.
type YAMLFormatter struct {
	writer io.Writer
}

// YAMLOption configures the YAML formatter.
type YAMLOption func(*YAMLFormatter)

// NewYAMLFormatter creates a new YAML formatter.
func NewYAMLFormatter(w io.Writer, opts ...YAMLOption) *YAMLFormatter {
	f := &YAMLFormatter{writer: w}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// Format writes the execution result as YAML.
func (f *YAMLFormatter) Format(result *execution.ExecutionResult) error {
	// Convert domain entity to output representation
	out := FromDomain(result)

	encoder := yaml.NewEncoder(f.writer, yaml.Indent(2))

	if err := encoder.Encode(out); err != nil {
		return err
	}

	return encoder.Close()
}
