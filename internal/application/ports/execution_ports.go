package ports

import (
	"context"
	"io"

	"github.com/reglet-dev/reglet/internal/application/dto"
	"github.com/reglet-dev/reglet/internal/domain/capabilities"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/execution"
)

// ExecutionEngine executes profiles and returns results.
type ExecutionEngine interface {
	Execute(ctx context.Context, profile entities.ProfileReader) (*execution.ExecutionResult, error)
	Close(ctx context.Context) error
}

// EngineFactory creates execution engines with capabilities.
type EngineFactory interface {
	CreateEngine(ctx context.Context, profile entities.ProfileReader, grantedCaps map[string][]capabilities.Capability, pluginDir string, filters dto.FilterOptions, execution dto.ExecutionOptions, skipSchemaValidation bool) (ExecutionEngine, error)
}

// OutputFormatter formats execution results.
type OutputFormatter interface {
	Format(result *execution.ExecutionResult) error
}

// FormatterOptions configures formatter behavior.
type FormatterOptions struct {
	ProfilePath string // For SARIF: reference to profile location
	Indent      bool   // For JSON: pretty-print with indentation
}

// OutputFormatterFactory creates formatters by name.
type OutputFormatterFactory interface {
	// Create returns a formatter for the given format name.
	// Returns error if format is unknown.
	Create(format string, writer io.Writer, options FormatterOptions) (OutputFormatter, error)

	// SupportedFormats returns list of available format names.
	SupportedFormats() []string
}

// OutputWriter writes formatted output to destination.
type OutputWriter interface {
	Write(ctx context.Context, data []byte, dest string) error
}

// Closer is a common interface for resources that need cleanup.
type Closer interface {
	io.Closer
}
