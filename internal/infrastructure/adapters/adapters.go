// Package adapters provides infrastructure adapters that implement application ports.
// These adapters wrap existing infrastructure components to satisfy port interfaces.
package adapters

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/expr-lang/expr"
	"github.com/whiskeyjimbo/reglet/internal/application/dto"
	"github.com/whiskeyjimbo/reglet/internal/application/ports"
	"github.com/whiskeyjimbo/reglet/internal/domain/capabilities"
	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
	"github.com/whiskeyjimbo/reglet/internal/domain/execution"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/build"
	infraconfig "github.com/whiskeyjimbo/reglet/internal/infrastructure/config"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/engine"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/redaction"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/system"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/validation"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/wasm"
)

// Ensure adapters implement ports at compile time
var (
	_ ports.ProfileLoader           = (*ProfileLoaderAdapter)(nil)
	_ ports.ProfileValidator        = (*ProfileValidatorAdapter)(nil)
	_ ports.SystemConfigProvider    = (*SystemConfigAdapter)(nil)
	_ ports.PluginDirectoryResolver = (*PluginDirectoryAdapter)(nil)
	_ ports.ExecutionEngine         = (*EngineAdapter)(nil)
	_ ports.EngineFactory           = (*EngineFactoryAdapter)(nil)
)

// ProfileLoaderAdapter adapts infrastructure profile loader to port interface.
type ProfileLoaderAdapter struct {
	loader      *infraconfig.ProfileLoader
	substitutor *infraconfig.VariableSubstitutor
}

// NewProfileLoaderAdapter creates a new profile loader adapter.
func NewProfileLoaderAdapter() *ProfileLoaderAdapter {
	return &ProfileLoaderAdapter{
		loader:      infraconfig.NewProfileLoader(),
		substitutor: infraconfig.NewVariableSubstitutor(),
	}
}

// LoadProfile loads and substitutes variables in a profile.
func (a *ProfileLoaderAdapter) LoadProfile(path string) (*entities.Profile, error) {
	profile, err := a.loader.LoadProfile(path)
	if err != nil {
		return nil, err
	}

	// Apply variable substitution
	if err := a.substitutor.Substitute(profile); err != nil {
		return nil, fmt.Errorf("variable substitution failed: %w", err)
	}

	return profile, nil
}

// ProfileValidatorAdapter adapts infrastructure validator to port interface.
type ProfileValidatorAdapter struct {
	validator *validation.ProfileValidator
}

// NewProfileValidatorAdapter creates a new profile validator adapter.
func NewProfileValidatorAdapter() *ProfileValidatorAdapter {
	return &ProfileValidatorAdapter{
		validator: validation.NewProfileValidator(),
	}
}

// Validate validates profile structure.
func (a *ProfileValidatorAdapter) Validate(profile *entities.Profile) error {
	return a.validator.Validate(profile)
}

// ValidateWithSchemas validates observation configs against plugin schemas.
func (a *ProfileValidatorAdapter) ValidateWithSchemas(ctx context.Context, profile *entities.Profile, runtime *wasm.Runtime) error {
	return a.validator.ValidateWithSchemas(ctx, profile, runtime)
}

// SystemConfigAdapter adapts system config loader to port interface.
type SystemConfigAdapter struct {
	loader *system.ConfigLoader
}

// NewSystemConfigAdapter creates a new system config adapter.
func NewSystemConfigAdapter() *SystemConfigAdapter {
	return &SystemConfigAdapter{
		loader: system.NewConfigLoader(),
	}
}

// LoadConfig loads system configuration from path.
func (a *SystemConfigAdapter) LoadConfig(ctx context.Context, path string) (*system.Config, error) {
	if path == "" {
		// Load from default location
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		path = filepath.Join(homeDir, ".reglet", "config.yaml")
	}

	return a.loader.Load(path)
}

// PluginDirectoryAdapter resolves plugin directory paths.
type PluginDirectoryAdapter struct{}

// NewPluginDirectoryAdapter creates a new plugin directory adapter.
func NewPluginDirectoryAdapter() *PluginDirectoryAdapter {
	return &PluginDirectoryAdapter{}
}

// ResolvePluginDir determines the plugin directory.
func (a *PluginDirectoryAdapter) ResolvePluginDir(ctx context.Context) (string, error) {
	// Try current working directory first
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	pluginDir := filepath.Join(cwd, "plugins")
	if _, err := os.Stat(pluginDir); err == nil {
		return pluginDir, nil
	}

	// Fallback to executable directory
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}

	exeDir := filepath.Dir(exePath)
	pluginDir = filepath.Join(exeDir, "..", "plugins")
	if _, err := os.Stat(pluginDir); err == nil {
		return pluginDir, nil
	}

	return "", fmt.Errorf("plugin directory not found in %s or %s", cwd, exeDir)
}

// EngineAdapter wraps infrastructure engine to implement port interface.
type EngineAdapter struct {
	engine *engine.Engine
}

// Execute executes the profile using the wrapped engine.
func (a *EngineAdapter) Execute(ctx context.Context, profile entities.ProfileReader) (*execution.ExecutionResult, error) {
	return a.engine.Execute(ctx, profile)
}

// Close closes the wrapped engine.
func (a *EngineAdapter) Close(ctx context.Context) error {
	return a.engine.Close(ctx)
}

// EngineFactoryAdapter creates execution engines.
type EngineFactoryAdapter struct {
	redactor          *redaction.Redactor
	wasmMemoryLimitMB int
}

// NewEngineFactoryAdapter creates a new engine factory adapter.
func NewEngineFactoryAdapter(redactor *redaction.Redactor, wasmMemoryLimitMB int) *EngineFactoryAdapter {
	return &EngineFactoryAdapter{
		redactor:          redactor,
		wasmMemoryLimitMB: wasmMemoryLimitMB,
	}
}

// CreateEngine creates an execution engine with capabilities.
func (a *EngineFactoryAdapter) CreateEngine(
	ctx context.Context,
	profile entities.ProfileReader,
	grantedCaps map[string][]capabilities.Capability,
	pluginDir string,
	filters dto.FilterOptions,
	execution dto.ExecutionOptions,
	skipSchemaValidation bool,
) (ports.ExecutionEngine, error) {
	// Create capability manager that uses the granted capabilities
	capMgr := &staticCapabilityManager{granted: grantedCaps}

	// Build execution config from filters and execution options
	cfg := a.buildExecutionConfig(filters, execution)

	// Create engine
	eng, err := engine.NewEngineWithCapabilities(
		ctx,
		build.Get(),
		capMgr,
		pluginDir,
		profile,
		cfg,
		a.redactor,
		nil, // No persistence
		a.wasmMemoryLimitMB,
	)
	if err != nil {
		return nil, err
	}

	return &EngineAdapter{engine: eng}, nil
}

// buildExecutionConfig constructs an ExecutionConfig from filter and execution options.
func (a *EngineFactoryAdapter) buildExecutionConfig(filters dto.FilterOptions, exec dto.ExecutionOptions) engine.ExecutionConfig {
	cfg := engine.DefaultExecutionConfig()

	// Apply execution options
	cfg.Parallel = exec.Parallel
	if exec.MaxConcurrentControls > 0 {
		cfg.MaxConcurrentControls = exec.MaxConcurrentControls
	}
	if exec.MaxConcurrentObservations > 0 {
		cfg.MaxConcurrentObservations = exec.MaxConcurrentObservations
	}

	// Apply filters
	cfg.IncludeTags = filters.IncludeTags
	cfg.IncludeSeverities = filters.IncludeSeverities
	cfg.IncludeControlIDs = filters.IncludeControlIDs
	cfg.ExcludeTags = filters.ExcludeTags
	cfg.ExcludeControlIDs = filters.ExcludeControlIDs
	cfg.IncludeDependencies = filters.IncludeDependencies

	// Compile filter expression if provided
	if filters.FilterExpression != "" {
		program, err := expr.Compile(filters.FilterExpression)
		if err != nil {
			// Log warning but don't fail - validation should have caught this earlier
			slog.Warn("failed to compile filter expression", "expression", filters.FilterExpression, "error", err)
		} else {
			cfg.FilterProgram = program
		}
	}

	return cfg
}

// staticCapabilityManager provides pre-granted capabilities.
type staticCapabilityManager struct {
	granted map[string][]capabilities.Capability
}

func (m *staticCapabilityManager) CollectRequiredCapabilities(
	ctx context.Context,
	profile entities.ProfileReader,
	runtime *wasm.Runtime,
	pluginDir string,
) (map[string][]capabilities.Capability, error) {
	// Return the pre-granted capabilities
	return m.granted, nil
}

func (m *staticCapabilityManager) GrantCapabilities(
	required map[string][]capabilities.Capability,
) (map[string][]capabilities.Capability, error) {
	// Return what was already granted
	return m.granted, nil
}
