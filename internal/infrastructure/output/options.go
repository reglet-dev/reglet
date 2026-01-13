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

// applyBaseOptions applies common options to a config.
func applyBaseOptions(opts []BaseOutputOption) baseOutputConfig {
	cfg := baseOutputConfig{indent: true} // Default: indented
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// FactoryOptions holds CLI-provided options for formatter creation.
// This is an infrastructure concern, not exposed through ports.
type FactoryOptions struct {
	ProfilePath string
	Indent      bool
	Verbose     bool
	NoColor     bool
	EvidenceDir string // For OSCAL (Phase 4)
}
