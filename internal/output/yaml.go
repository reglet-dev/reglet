package output

import (
	"io"

	"github.com/whiskeyjimbo/reglet/internal/engine"
	"gopkg.in/yaml.v3"
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
func (f *YAMLFormatter) Format(result *engine.ExecutionResult) error {
	encoder := yaml.NewEncoder(f.writer)
	encoder.SetIndent(2)

	if err := encoder.Encode(result); err != nil {
		return err
	}

	return encoder.Close()
}
