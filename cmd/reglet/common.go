package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// CommonOptions contains flags shared across all commands.
type CommonOptions struct {
	// Output
	Format string

	// Execution
	Timeout time.Duration

	// Flags (bools grouped for alignment)
	Parallel bool
	Verbose  bool
	Quiet    bool

	// Future: add more as needed
	// CacheDir    string
	// PluginDir   string
	// ConfigPath  string
}

// DefaultCommonOptions returns sensible defaults.
func DefaultCommonOptions() CommonOptions {
	return CommonOptions{
		Timeout:  2 * time.Minute,
		Format:   "table",
		Parallel: true,
	}
}

// RegisterFlags adds common flags to a cobra command.
func (opts *CommonOptions) RegisterFlags(cmd *cobra.Command) {
	// Execution
	cmd.Flags().DurationVar(&opts.Timeout, "timeout", opts.Timeout,
		"Global timeout for entire execution (0 to disable)")
	cmd.Flags().BoolVar(&opts.Parallel, "parallel", opts.Parallel,
		"Enable parallel execution")

	// Output
	cmd.Flags().StringVar(&opts.Format, "format", opts.Format,
		"Output format: table, json, yaml, junit, sarif")
	cmd.Flags().BoolVarP(&opts.Verbose, "verbose", "v", false,
		"Verbose output")
	cmd.Flags().BoolVarP(&opts.Quiet, "quiet", "q", false,
		"Quiet output (errors only)")
}

// ApplyToContext applies timeout to context.
// Returns new context and cancel function.
func (opts *CommonOptions) ApplyToContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if opts.Timeout > 0 {
		return context.WithTimeout(ctx, opts.Timeout)
	}
	// No timeout - return no-op cancel
	return ctx, func() {}
}

// ValidateFlags validates common options.
func (opts *CommonOptions) ValidateFlags() error {
	if opts.Verbose && opts.Quiet {
		return fmt.Errorf("--verbose and --quiet are mutually exclusive")
	}

	validFormats := map[string]bool{
		"table": true, "json": true, "yaml": true,
		"junit": true, "sarif": true,
	}
	if !validFormats[opts.Format] {
		return fmt.Errorf("invalid format: %s (valid: table, json, yaml, junit, sarif)", opts.Format)
	}

	return nil
}
