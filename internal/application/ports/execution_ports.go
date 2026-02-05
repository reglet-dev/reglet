package ports

import (
	"context"
	"io"

	sdkEntities "github.com/reglet-dev/reglet-sdk/domain/entities"
	"github.com/reglet-dev/reglet/internal/application/dto"
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
	CreateEngine(ctx context.Context, profile entities.ProfileReader, grantedCaps map[string]*sdkEntities.GrantSet, pluginDir string, filters dto.FilterOptions, execution dto.ExecutionOptions, skipSchemaValidation bool) (ExecutionEngine, error)
}

// OutputFormatter formats execution results.
type OutputFormatter interface {
	Format(result *execution.ExecutionResult) error
}

// OutputFormatterFactory creates formatters by name.
// The options parameter is implementation-specific (e.g., output.FactoryOptions).
type OutputFormatterFactory interface {
	// Create returns a formatter for the given format name.
	// options is implementation-specific (typically output.FactoryOptions).
	// Returns error if format is unknown.
	Create(format string, writer io.Writer, options interface{}) (OutputFormatter, error)

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
