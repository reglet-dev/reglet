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
// options should be of type FactoryOptions or nil.
func (f *FormatterFactory) Create(
	format string,
	writer io.Writer,
	options interface{},
) (ports.OutputFormatter, error) {
	// Type assert options (default to empty if nil)
	opts := FactoryOptions{}
	if options != nil {
		if o, ok := options.(FactoryOptions); ok {
			opts = o
		}
	}

	switch format {
	case "table":
		return NewTableFormatter(writer,
			WithNoColor(opts.NoColor),
			WithShowDetails(opts.ShowDetails),
		), nil

	case "json":
		return NewJSONFormatter(writer,
			WithJSONIndent(opts.Indent),
		), nil

	case "yaml":
		return NewYAMLFormatter(writer), nil

	case "junit":
		return NewJUnitFormatter(writer), nil

	case "sarif":
		return NewSARIFFormatter(writer,
			WithProfilePath(opts.ProfilePath),
		), nil

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
