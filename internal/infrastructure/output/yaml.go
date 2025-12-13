package output

import (
	"io"

	"github.com/goccy/go-yaml"
	"github.com/whiskeyjimbo/reglet/internal/domain/execution"
)

// YAMLFormatter formats execution results as YAML.
type YAMLFormatter struct {
	writer io.Writer
}

// NewYAMLFormatter creates a new YAML formatter.
func NewYAMLFormatter(w io.Writer) *YAMLFormatter {
	return &YAMLFormatter{writer: w}
}

// Format writes the execution result as YAML.
func (f *YAMLFormatter) Format(result *execution.ExecutionResult) error {
	encoder := yaml.NewEncoder(f.writer, yaml.Indent(2))

	if err := encoder.Encode(result); err != nil {
		return err
	}

	return encoder.Close()
}
