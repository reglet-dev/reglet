package output

import (
	"fmt"
	"io"

	"github.com/reglet-dev/reglet/internal/application/ports"
)

// FormatterFactory implements ports.OutputFormatterFactory.
type FormatterFactory struct{}

// NewFormatterFactory creates a new formatter factory.
func NewFormatterFactory() *FormatterFactory {
	return &FormatterFactory{}
}

// Create returns a formatter for the given format name.
func (f *FormatterFactory) Create(
	format string,
	writer io.Writer,
	options ports.FormatterOptions,
) (ports.OutputFormatter, error) {
	switch format {
	case "table":
		return NewTableFormatter(writer), nil
	case "json":
		return NewJSONFormatter(writer, options.Indent), nil
	case "yaml":
		return NewYAMLFormatter(writer), nil
	case "junit":
		return NewJUnitFormatter(writer), nil
	case "sarif":
		return NewSARIFFormatter(writer, options.ProfilePath), nil
	default:
		return nil, fmt.Errorf(
			"unknown format: %s (supported: %v)",
			format, f.SupportedFormats(),
		)
	}
}

// SupportedFormats returns list of available format names.
func (f *FormatterFactory) SupportedFormats() []string {
	return []string{"table", "json", "yaml", "junit", "sarif"}
}
