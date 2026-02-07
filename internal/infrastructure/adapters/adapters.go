// Package adapters provides infrastructure adapters that implement application ports.
// These adapters wrap existing infrastructure components to satisfy port interfaces.
package adapters

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/expr-lang/expr"
	abi "github.com/reglet-dev/reglet-abi"
	"github.com/reglet-dev/reglet/internal/application/dto"
	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/domain/capability"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/execution"
	"github.com/reglet-dev/reglet/internal/infrastructure/build"
	infraconfig "github.com/reglet-dev/reglet/internal/infrastructure/config"
	"github.com/reglet-dev/reglet/internal/infrastructure/engine"
	"github.com/reglet-dev/reglet/internal/infrastructure/sensitivedata"
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
	redactor *sensitivedata.Redactor
	version  build.Info
}

// NewPluginRuntimeFactoryAdapter creates a new runtime factory adapter.
func NewPluginRuntimeFactoryAdapter(redactor *sensitivedata.Redactor) *PluginRuntimeFactoryAdapter {
	return &PluginRuntimeFactoryAdapter{
		version:  build.Get(),
		redactor: redactor,
	}
}

// NewRuntime creates a new plugin runtime with optional configuration.
// Accepts functional options from wasm package (automatically adds redactor from factory).
func (f *PluginRuntimeFactoryAdapter) NewRuntime(ctx context.Context, opts ...ports.RuntimeOption) (ports.PluginRuntime, error) {
	// Convert ports.RuntimeOption to wasm.RuntimeOption
	wasmOpts := make([]wasm.RuntimeOption, 0, len(opts)+1)
	for _, opt := range opts {
		if wasmOpt, ok := opt.(wasm.RuntimeOption); ok {
			wasmOpts = append(wasmOpts, wasmOpt)
		}
	}

	// Always add redactor if configured
	if f.redactor != nil {
		wasmOpts = append(wasmOpts, wasm.WithRedactor(f.redactor))
	}

	runtime, err := wasm.NewRuntime(ctx, f.version, wasmOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create runtime: %w", err)
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

// Manifest returns plugin metadata.
func (p *PluginAdapter) Manifest(ctx context.Context) (*abi.Manifest, error) {
	return p.plugin.Manifest(ctx)
}

// RequiredCapabilities returns plugin declared capabilities.
func (p *PluginAdapter) RequiredCapabilities(ctx context.Context) (capability.GrantSet, error) {
	return p.plugin.RequiredCapabilities(ctx)
}

// ProfileLoaderAdapter adapts infrastructure profile loader to port interface.
type ProfileLoaderAdapter struct {
	loader        *infraconfig.ProfileLoader
	substitutor   *infraconfig.VariableSubstitutor
	remoteFetcher RemoteProfileFetcher
}

// RemoteProfileFetcher is the interface for fetching remote profiles.
// This allows the adapter to fetch profiles from URLs without depending on
// the concrete implementation from application/services.
type RemoteProfileFetcher interface {
	FetchAsReader(ctx context.Context, url string, opts RemoteFetchOptions) (io.Reader, error)
}

// RemoteFetchOptions configures remote profile fetching.
type RemoteFetchOptions struct {
	Headers             map[string]string
	Timeout             time.Duration
	Refresh             bool
	AllowPrivateNetwork bool
	Insecure            bool
}

// ProfileLoaderOption configures a ProfileLoaderAdapter.
type ProfileLoaderOption func(*ProfileLoaderAdapter)

// WithRemoteFetcher sets the remote profile fetcher.
func WithRemoteFetcher(fetcher RemoteProfileFetcher) ProfileLoaderOption {
	return func(a *ProfileLoaderAdapter) { a.remoteFetcher = fetcher }
}

// NewProfileLoaderAdapter creates a new profile loader adapter.
func NewProfileLoaderAdapter(resolver ports.SecretResolver, opts ...ProfileLoaderOption) *ProfileLoaderAdapter {
	a := &ProfileLoaderAdapter{
		loader:      infraconfig.NewProfileLoader(),
		substitutor: infraconfig.NewVariableSubstitutor(resolver),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// LoadProfile loads and substitutes variables in a profile.
func (a *ProfileLoaderAdapter) LoadProfile(path string) (*entities.Profile, error) {
	return a.LoadProfileWithCLIVars(path, nil)
}

// LoadProfileWithCLIVars loads a profile and merges CLI variables before substitution.
// CLI variables override profile variables at the same path.
// Supports both local file paths and remote URLs (https://, oci://).
func (a *ProfileLoaderAdapter) LoadProfileWithCLIVars(path string, cliVars map[string]interface{}) (*entities.Profile, error) {
	return a.LoadProfileWithOptions(path, cliVars, ports.RemoteLoadOptions{})
}

// LoadProfileWithOptions loads a profile with CLI variables and remote fetch options.
// remoteOpts configures behavior for remote profile fetching (refresh, timeout, etc.).
func (a *ProfileLoaderAdapter) LoadProfileWithOptions(path string, cliVars map[string]interface{}, remoteOpts ports.RemoteLoadOptions) (*entities.Profile, error) {
	var profile *entities.Profile
	var err error

	// Check if path is a remote URL
	if isRemoteProfile(path) {
		profile, err = a.loadRemoteProfile(path, remoteOpts)
	} else {
		profile, err = a.loader.LoadProfile(path)
	}

	if err != nil {
		return nil, err
	}

	// Merge CLI variables into profile vars (CLI wins)
	if len(cliVars) > 0 {
		profile.Vars = infraconfig.MergeCLIVars(profile.Vars, cliVars)
	}

	// Apply variable substitution with merged vars
	if err := a.substitutor.Substitute(profile); err != nil {
		return nil, fmt.Errorf("variable substitution failed: %w", err)
	}

	return profile, nil
}

// loadRemoteProfile fetches and loads a profile from a remote URL.
func (a *ProfileLoaderAdapter) loadRemoteProfile(url string, opts ports.RemoteLoadOptions) (*entities.Profile, error) {
	if a.remoteFetcher == nil {
		return nil, fmt.Errorf("remote profile fetching not configured; cannot load %s", url)
	}

	// Fetch the remote profile content with the provided options
	ctx := context.Background()
	fetchOpts := RemoteFetchOptions{
		Refresh:             opts.Refresh,
		AllowPrivateNetwork: opts.AllowPrivateNetwork,
		Insecure:            opts.Insecure,
		Timeout:             opts.Timeout,
	}
	reader, err := a.remoteFetcher.FetchAsReader(ctx, url, fetchOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch remote profile: %w", err)
	}

	// Parse the fetched content
	return a.loader.LoadProfileFromReader(reader)
}

// isRemoteProfile returns true if the path looks like a remote URL.
func isRemoteProfile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "oci://")
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

	// 1. Try local ./plugins
	pluginDir := filepath.Join(cwd, "plugins")
	if _, err := os.Stat(pluginDir); err == nil {
		return pluginDir, nil
	}

	// 2. Try sibling reglet-plugins/plugins (new structure)
	// Walk up to find go.mod, then look for sibling
	projectRoot := findProjectRoot()
	siblingPlugins := filepath.Join(filepath.Dir(projectRoot), "reglet-plugins", "plugins")
	if _, err := os.Stat(siblingPlugins); err == nil {
		return siblingPlugins, nil
	}

	// 3. Fallback to executable directory
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
	redactor *sensitivedata.Redactor
	runtime  *infraconfig.RuntimeConfig
}

// NewEngineFactoryAdapter creates a new engine factory adapter.
func NewEngineFactoryAdapter(redactor *sensitivedata.Redactor, runtime *infraconfig.RuntimeConfig) *EngineFactoryAdapter {
	return &EngineFactoryAdapter{
		redactor: redactor,
		runtime:  runtime,
	}
}

// CreateEngine creates an execution engine with capabilities.
func (a *EngineFactoryAdapter) CreateEngine(
	ctx context.Context,
	profile entities.ProfileReader,
	grantedCaps map[string]capability.GrantSet,
	pluginDir string,
	filters dto.FilterOptions,
	exec dto.ExecutionOptions,
	_ bool, // skipSchemaValidation - reserved for future schema validation feature
) (ports.ExecutionEngine, error) {
	// Create capability manager that uses the granted capabilities
	capMgr := &staticCapabilityManager{granted: grantedCaps}

	// Build execution config from filters and execution options
	cfg := a.buildExecutionConfig(filters, exec)

	// Create engine
	eng, err := engine.NewEngine(
		ctx,
		build.Get(),
		engine.WithCapabilities(grantedCaps), // Pass pre-granted capabilities directly
		engine.WithCapabilityManager(capMgr),
		engine.WithPluginDir(pluginDir),
		engine.WithProfile(profile),
		engine.WithExecutionConfig(cfg),
		engine.WithRedactor(a.redactor),
		engine.WithMemoryLimit(a.runtime.WasmMemoryLimitMB),
		engine.WithTruncator(&execution.GreedyTruncator{}),
	)
	if err != nil {
		return nil, err
	}

	return &EngineAdapter{engine: eng}, nil
}

// buildExecutionConfig constructs an ExecutionConfig from filter and execution options.
func (a *EngineFactoryAdapter) buildExecutionConfig(filters dto.FilterOptions, exec dto.ExecutionOptions) engine.ExecutionConfig {
	cfg := engine.DefaultExecutionConfig()

	// Apply runtime config defaults
	cfg.MaxEvidenceSizeBytes = a.runtime.MaxEvidenceSizeBytes
	cfg.MaxConcurrentControls = a.runtime.MaxConcurrentControls
	cfg.MaxConcurrentObservations = a.runtime.MaxConcurrentObservations

	// Apply execution options overrides if set
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
	granted map[string]capability.GrantSet
}

func (m *staticCapabilityManager) CollectRequiredCapabilities(
	_ context.Context,
	_ entities.ProfileReader,
	_ *wasm.Runtime,
	_ string,
) (map[string]capability.GrantSet, error) {
	// Return the pre-granted capabilities
	return m.granted, nil
}

func (m *staticCapabilityManager) GrantCapabilities(
	_ map[string]capability.GrantSet,
) (map[string]capability.GrantSet, error) {
	// Return what was already granted
	return m.granted, nil
}

// findProjectRoot attempts to find the project root by looking for the go.mod file.
func findProjectRoot() string {
	workingDir, err := os.Getwd()
	if err != nil {
		return "."
	}

	currentDir := workingDir
	for i := 0; i < 5; i++ {
		if _, err := os.Stat(filepath.Join(currentDir, "go.mod")); err == nil {
			return currentDir
		}
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			break
		}
		currentDir = parentDir
	}

	return workingDir
}
