package output

// baseOutputConfig holds options common to all formatters.
type baseOutputConfig struct {
	indent bool
}

// BaseOutputOption configures common formatter behavior.
type BaseOutputOption func(*baseOutputConfig)

// WithIndent enables indented/pretty-printed output.
func WithIndent(enabled bool) BaseOutputOption {
	return func(c *baseOutputConfig) {
		c.indent = enabled
	}
}

// FactoryOptions holds CLI-provided options for formatter creation.
// This is an infrastructure concern, not exposed through ports.
type FactoryOptions struct {
	ProfilePath string
	EvidenceDir string
	Indent      bool
	Verbose     bool
	NoColor     bool
	ShowDetails bool // Show detailed evidence for loop observations
}
