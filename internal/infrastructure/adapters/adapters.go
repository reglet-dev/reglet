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
	"github.com/reglet-dev/reglet/internal/application/dto"
	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/domain/capabilities"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/execution"
	"github.com/reglet-dev/reglet/internal/infrastructure/build"
	infraconfig "github.com/reglet-dev/reglet/internal/infrastructure/config"
	"github.com/reglet-dev/reglet/internal/infrastructure/engine"
	"github.com/reglet-dev/reglet/internal/infrastructure/redaction"
	"github.com/reglet-dev/reglet/internal/infrastructure/system"
	"github.com/reglet-dev/reglet/internal/infrastructure/validation"
	"github.com/reglet-dev/reglet/internal/infrastructure/wasm"
)

// Ensure adapters implement ports at compile time
var (
	_ ports.ProfileLoader           = (*ProfileLoaderAdapter)(nil)
	_ ports.ProfileValidator        = (*ProfileValidatorAdapter)(nil)
	_ ports.SystemConfigProvider    = (*SystemConfigAdapter)(nil)
	_ ports.PluginDirectoryResolver = (*PluginDirectoryAdapter)(nil)
	_ ports.ExecutionEngine         = (*EngineAdapter)(nil)
	_ ports.EngineFactory           = (*EngineFactoryAdapter)(nil)
	_ ports.PluginRuntimeFactory    = (*PluginRuntimeFactoryAdapter)(nil)
	_ ports.PluginRuntime           = (*PluginRuntimeAdapter)(nil)
	_ ports.Plugin                  = (*PluginAdapter)(nil)
)

// PluginRuntimeFactoryAdapter creates PluginRuntime instances.
// This adapter decouples the application layer from the concrete wasm.Runtime.
type PluginRuntimeFactoryAdapter struct {
	redactor *redaction.Redactor
	version  build.Info
}

// NewPluginRuntimeFactoryAdapter creates a new runtime factory adapter.
func NewPluginRuntimeFactoryAdapter(redactor *redaction.Redactor) *PluginRuntimeFactoryAdapter {
	return &PluginRuntimeFactoryAdapter{
		version:  build.Get(),
		redactor: redactor,
	}
}

// NewRuntime creates a new plugin runtime for capability collection.
func (f *PluginRuntimeFactoryAdapter) NewRuntime(ctx context.Context) (ports.PluginRuntime, error) {
	runtime, err := wasm.NewRuntime(ctx, f.version)
	if err != nil {
		return nil, fmt.Errorf("failed to create runtime: %w", err)
	}
	return &PluginRuntimeAdapter{runtime: runtime}, nil
}

// NewRuntimeWithCapabilities creates a runtime with granted capabilities.
func (f *PluginRuntimeFactoryAdapter) NewRuntimeWithCapabilities(
	ctx context.Context,
	caps map[string][]capabilities.Capability,
	memoryLimitMB int,
) (ports.PluginRuntime, error) {
	runtime, err := wasm.NewRuntimeWithCapabilities(ctx, f.version, caps, f.redactor, memoryLimitMB)
	if err != nil {
		return nil, fmt.Errorf("failed to create runtime with capabilities: %w", err)
	}
	return &PluginRuntimeAdapter{runtime: runtime}, nil
}

// PluginRuntimeAdapter wraps wasm.Runtime to implement ports.PluginRuntime.
type PluginRuntimeAdapter struct {
	runtime *wasm.Runtime
}

// LoadPlugin loads a plugin from WASM bytes.
func (r *PluginRuntimeAdapter) LoadPlugin(ctx context.Context, name string, wasmBytes []byte) (ports.Plugin, error) {
	plugin, err := r.runtime.LoadPlugin(ctx, name, wasmBytes)
	if err != nil {
		return nil, err
	}
	return &PluginAdapter{plugin: plugin}, nil
}

// Close releases runtime resources.
func (r *PluginRuntimeAdapter) Close(ctx context.Context) error {
	return r.runtime.Close(ctx)
}

// UnwrapRuntime returns the underlying wasm.Runtime for infrastructure-layer use.
// This should only be used by infrastructure code that needs the concrete type.
func (r *PluginRuntimeAdapter) UnwrapRuntime() *wasm.Runtime {
	return r.runtime
}

// PluginAdapter wraps wasm.Plugin to implement ports.Plugin.
type PluginAdapter struct {
	plugin *wasm.Plugin
}

// Describe returns plugin metadata.
func (p *PluginAdapter) Describe(ctx context.Context) (*ports.PluginInfo, error) {
	info, err := p.plugin.Describe(ctx)
	if err != nil {
		return nil, err
	}
	return &ports.PluginInfo{
		Name:         info.Name,
		Version:      info.Version,
		Description:  info.Description,
		Capabilities: info.Capabilities,
	}, nil
}

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
func (a *ProfileValidatorAdapter) ValidateWithSchemas(ctx context.Context, profile *entities.Profile, runtime ports.PluginRuntime) error {
	// Unwrap to get concrete runtime if possible
	if adapter, ok := runtime.(*PluginRuntimeAdapter); ok {
		return a.validator.ValidateWithSchemas(ctx, profile, adapter.UnwrapRuntime())
	}
	// For mock runtimes in tests, skip schema validation
	return nil
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
func (a *SystemConfigAdapter) LoadConfig(_ context.Context, path string) (*system.Config, error) {
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
func (a *PluginDirectoryAdapter) ResolvePluginDir(_ context.Context) (string, error) {
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
	_ bool, // skipSchemaValidation - reserved for future schema validation feature
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
	_ context.Context,
	_ entities.ProfileReader,
	_ *wasm.Runtime,
	_ string,
) (map[string][]capabilities.Capability, error) {
	// Return the pre-granted capabilities
	return m.granted, nil
}

func (m *staticCapabilityManager) GrantCapabilities(
	_ map[string][]capabilities.Capability,
) (map[string][]capabilities.Capability, error) {
	// Return what was already granted
	return m.granted, nil
}
