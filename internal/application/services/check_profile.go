// Package services contains application use cases.
package services

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/expr-lang/expr"
	"github.com/reglet-dev/reglet/internal/application/dto"
	apperrors "github.com/reglet-dev/reglet/internal/application/errors"
	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/domain/capabilities"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/execution"
	"github.com/reglet-dev/reglet/internal/domain/services"
)

// CheckProfileUseCase orchestrates the complete profile check workflow.
// This is a pure application layer component that depends only on ports.
type CheckProfileUseCase struct {
	profileLoader    ports.ProfileLoader
	profileCompiler  *services.ProfileCompiler
	profileValidator ports.ProfileValidator
	systemConfig     ports.SystemConfigProvider
	pluginResolver   ports.PluginDirectoryResolver
	capOrchestrator  *CapabilityOrchestrator
	lockfileService  *LockfileService
	pluginService    *PluginService
	engineFactory    ports.EngineFactory
	logger           *slog.Logger
}

// NewCheckProfileUseCase creates a new check profile use case.
func NewCheckProfileUseCase(
	profileLoader ports.ProfileLoader,
	profileCompiler *services.ProfileCompiler,
	profileValidator ports.ProfileValidator,
	systemConfig ports.SystemConfigProvider,
	pluginResolver ports.PluginDirectoryResolver,
	capOrchestrator *CapabilityOrchestrator,
	lockfileService *LockfileService,
	pluginService *PluginService,
	engineFactory ports.EngineFactory,
	logger *slog.Logger,
) *CheckProfileUseCase {
	if logger == nil {
		logger = slog.Default()
	}

	return &CheckProfileUseCase{
		profileLoader:    profileLoader,
		profileCompiler:  profileCompiler,
		profileValidator: profileValidator,
		systemConfig:     systemConfig,
		pluginResolver:   pluginResolver,
		capOrchestrator:  capOrchestrator,
		lockfileService:  lockfileService,
		pluginService:    pluginService,
		engineFactory:    engineFactory,
		logger:           logger,
	}
}

// Execute runs the complete check profile workflow.
func (uc *CheckProfileUseCase) Execute(ctx context.Context, req dto.CheckProfileRequest) (*dto.CheckProfileResponse, error) {
	startTime := time.Now()

	uc.logger.Info("loading profile", "path", req.ProfilePath)

	// 1-2. Load and compile (clean up imports, validation)
	profile, err := uc.loadAndCompileProfile(req.ProfilePath)
	if err != nil {
		return nil, err
	}

	uc.logger.Info("profile compiled and validated", "controls", profile.ControlCount())

	// 2b. Resolve/Lock plugins
	if err := uc.resolveAndLockPlugins(ctx, profile, req.ProfilePath); err != nil {
		return nil, err
	}

	// 3. Filters
	if err := uc.validateFilters(profile, req.Filters); err != nil {
		return nil, err
	}

	// 4. Plugin Dir (Legacy/Local resolution)
	localPluginDir, err := uc.resolvePluginDir(ctx, req.Options.PluginDir)
	if err != nil {
		uc.logger.Debug("failed to resolve local plugin directory", "error", err)
		localPluginDir = ""
	}

	// 4b. Validate Declared Plugins
	if err := uc.validateDeclaredPlugins(profile, localPluginDir); err != nil {
		return nil, err
	}

	// 5. Prepare Plugin Runtime Environment (Hybrid Local/OCI)
	// Creates a temporary directory with symlinks to all required plugins
	runtimePluginDir, cleanup, err := uc.preparePluginEnvironment(ctx, profile.Plugins, localPluginDir)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare plugin environment: %w", err)
	}
	defer cleanup()

	// 6-8. Prepare Engine using runtime dir
	eng, requiredCaps, grantedCaps, err := uc.prepareEngine(ctx, profile, runtimePluginDir, req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = eng.Close(ctx) }()

	// 9. Execute
	result, err := uc.executeProfile(ctx, eng, profile)
	if err != nil {
		return nil, err
	}

	// 10. Start Response
	return uc.buildResponse(req, startTime, result, requiredCaps, grantedCaps), nil
}

func (uc *CheckProfileUseCase) loadAndCompileProfile(path string) (*entities.ValidatedProfile, error) {
	rawProfile, err := uc.profileLoader.LoadProfile(path)
	if err != nil {
		return nil, apperrors.NewValidationError("profile", "failed to load profile", err.Error())
	}

	uc.logger.Info("profile loaded", "name", rawProfile.Metadata.Name, "version", rawProfile.Metadata.Version)

	profile, err := uc.profileCompiler.Compile(rawProfile)
	if err != nil {
		return nil, apperrors.NewValidationError("profile", "compilation failed", err.Error())
	}
	return profile, nil
}

func (uc *CheckProfileUseCase) resolveAndLockPlugins(
	ctx context.Context,
	profile *entities.ValidatedProfile,
	profilePath string,
) error {
	lockfilePath := filepath.Join(filepath.Dir(profilePath), "reglet.lock")
	lockfile, err := uc.lockfileService.ResolvePlugins(ctx, profile.Profile, lockfilePath)
	if err != nil {
		return apperrors.NewConfigurationError("lockfile", "failed to resolve plugins", err)
	}

	var strictPlugins []string
	for _, p := range profile.Plugins {
		spec, _ := entities.ParsePluginDeclaration(p) // Error checked in ResolvePlugins
		if locked := lockfile.GetPlugin(spec.Name); locked != nil {
			// Using simplified strict declaration: currently appending @resolved
			strictDecl := fmt.Sprintf("%s@%s", spec.Name, locked.Resolved)
			if spec.Name != spec.Source {
				strictDecl = fmt.Sprintf("%s@%s", spec.Source, locked.Resolved)
			}
			strictPlugins = append(strictPlugins, strictDecl)
		} else {
			strictPlugins = append(strictPlugins, p)
		}
	}
	profile.Plugins = strictPlugins

	uc.logger.Info("plugins resolved", "lockfile", lockfilePath)
	return nil
}

func (uc *CheckProfileUseCase) resolvePluginDir(ctx context.Context, override string) (string, error) {
	if override != "" {
		return override, nil
	}
	return uc.pluginResolver.ResolvePluginDir(ctx)
}

func (uc *CheckProfileUseCase) prepareEngine(
	ctx context.Context,
	profile *entities.ValidatedProfile,
	pluginDir string,
	req dto.CheckProfileRequest,
) (
	ports.ExecutionEngine,
	map[string][]capabilities.Capability,
	map[string][]capabilities.Capability,
	error,
) {
	requiredCaps, tempRuntime, err := uc.capOrchestrator.CollectCapabilities(ctx, profile, pluginDir)
	if err != nil {
		return nil, nil, nil, apperrors.NewConfigurationError("capabilities", "failed to collect capabilities", err)
	}
	if tempRuntime != nil {
		_ = tempRuntime.Close(ctx)
	}

	grantedCaps, err := uc.capOrchestrator.GrantCapabilities(requiredCaps, req.Options.TrustPlugins)
	if err != nil {
		return nil, nil, nil, apperrors.NewCapabilityError("capability grant failed", flattenCapabilities(requiredCaps))
	}

	eng, err := uc.engineFactory.CreateEngine(
		ctx,
		profile,
		grantedCaps,
		pluginDir,
		req.Filters,
		req.Execution,
		req.Options.SkipSchemaValidation,
	)
	if err != nil {
		return nil, nil, nil, apperrors.NewConfigurationError("engine", "failed to create engine", err)
	}

	return eng, requiredCaps, grantedCaps, nil
}

func (uc *CheckProfileUseCase) executeProfile(
	ctx context.Context,
	eng ports.ExecutionEngine,
	profile *entities.ValidatedProfile,
) (*execution.ExecutionResult, error) {
	uc.logger.Info("executing profile")
	result, err := eng.Execute(ctx, profile)
	if err != nil {
		return nil, apperrors.NewExecutionError("", "execution failed", err)
	}

	uc.logger.Info("execution complete",
		"duration", result.Duration,
		"total_controls", result.Summary.TotalControls,
		"passed", result.Summary.PassedControls,
		"failed", result.Summary.FailedControls,
		"errors", result.Summary.ErrorControls,
		"skipped", result.Summary.SkippedControls)
	return result, nil
}

func (uc *CheckProfileUseCase) buildResponse(
	req dto.CheckProfileRequest,
	startTime time.Time,
	result *execution.ExecutionResult,
	reqCaps, grantedCaps map[string][]capabilities.Capability,
) *dto.CheckProfileResponse {
	return &dto.CheckProfileResponse{
		ExecutionResult: result,
		Metadata: dto.ResponseMetadata{
			RequestID:   req.Metadata.RequestID,
			ProcessedAt: time.Now(),
			Duration:    time.Since(startTime),
		},
		Diagnostics: dto.Diagnostics{
			Capabilities: dto.CapabilityDiagnostics{
				Required: reqCaps,
				Granted:  grantedCaps,
			},
		},
	}
}

// validateFilters validates filter configuration and compiles filter expressions.
func (uc *CheckProfileUseCase) validateFilters(profile entities.ProfileReader, filters dto.FilterOptions) error {
	// If either include or exclude IDs are provided, build a map once
	if len(filters.IncludeControlIDs) > 0 || len(filters.ExcludeControlIDs) > 0 {
		controls := profile.GetAllControls()
		controlMap := make(map[string]bool, len(controls))
		for _, ctrl := range controls {
			controlMap[ctrl.ID] = true
		}

		// Validate --control references exist
		for _, id := range filters.IncludeControlIDs {
			if !controlMap[id] {
				return apperrors.NewValidationError(
					"filters",
					fmt.Sprintf("--control references non-existent control: %s", id),
				)
			}
		}

		// Validate --exclude-control references exist
		for _, id := range filters.ExcludeControlIDs {
			if !controlMap[id] {
				return apperrors.NewValidationError(
					"filters",
					fmt.Sprintf("--exclude-control references non-existent control: %s", id),
				)
			}
		}
	}

	// Compile filter expression if provided
	if filters.FilterExpression != "" {
		_, err := expr.Compile(filters.FilterExpression,
			expr.Env(services.ControlEnv{}),
			expr.AsBool())
		if err != nil {
			return apperrors.NewValidationError(
				"filters",
				fmt.Sprintf("invalid --filter expression: %v\nExample: severity in ['critical', 'high'] && !('slow' in tags)", err),
			)
		}
	}

	return nil
}

// builtInPlugins lists plugins that are embedded in the reglet binary.
var builtInPlugins = map[string]bool{
	"file":    true,
	"http":    true,
	"dns":     true,
	"tcp":     true,
	"smtp":    true,
	"command": true,
}

// validateDeclaredPlugins validates that declared plugins exist and all used plugins are declared.
// This enforces explicit dependency declaration during development.
func (uc *CheckProfileUseCase) validateDeclaredPlugins(profile entities.ProfileReader, pluginDir string) error {
	declaredPlugins := profile.GetPlugins()
	usedPlugins := uc.getUsedPlugins(profile)

	// Build set of declared plugin names for lookup
	declaredSet := make(map[string]bool)
	for _, declared := range declaredPlugins {
		declaredSet[extractPluginName(declared)] = true
	}

	// 1. Check if used plugins are declared
	if err := uc.checkMissingDeclarations(declaredPlugins, usedPlugins, declaredSet); err != nil {
		return err
	}

	// 2. Verify existence of declared plugins
	return uc.verifyPluginExistence(declaredPlugins, pluginDir)
}

func (uc *CheckProfileUseCase) getUsedPlugins(profile entities.ProfileReader) map[string]bool {
	usedPlugins := make(map[string]bool)
	for _, ctrl := range profile.GetAllControls() {
		for _, obs := range ctrl.ObservationDefinitions {
			usedPlugins[obs.Plugin] = true
		}
	}
	return usedPlugins
}

func (uc *CheckProfileUseCase) checkMissingDeclarations(declared []string, used map[string]bool, declaredSet map[string]bool) error {
	// Require plugins field if any observations use plugins
	if len(declared) == 0 && len(used) > 0 {
		var pluginList []string
		for p := range used {
			pluginList = append(pluginList, p)
		}
		return apperrors.NewValidationError(
			"plugins",
			fmt.Sprintf("plugins field is required; add 'plugins:' section declaring: %v", pluginList),
		)
	}

	// Error if plugins are used but not declared
	for p := range used {
		if !declaredSet[p] {
			return apperrors.NewValidationError(
				"plugins",
				fmt.Sprintf("plugin %q used in observations but not declared in 'plugins:' section", p),
			)
		}
	}
	return nil
}

func (uc *CheckProfileUseCase) verifyPluginExistence(declared []string, pluginDir string) error {
	for _, rawDecl := range declared {
		// Extract plugin name from path if it's a path (e.g., ./plugins/file/file.wasm -> file)
		pluginName := extractPluginName(rawDecl)

		// Check if it's a built-in plugin
		if builtInPlugins[pluginName] {
			continue
		}

		// Check if external plugin exists on filesystem
		if pluginDir != "" {
			pluginPath := filepath.Join(pluginDir, pluginName, pluginName+".wasm")
			if _, err := os.Stat(pluginPath); err == nil {
				continue
			}
		}

		// Check if declared path exists directly (for ./plugins/... format)
		if strings.HasPrefix(rawDecl, "./") || strings.HasPrefix(rawDecl, "/") {
			if _, err := os.Stat(rawDecl); err == nil {
				continue
			}
		}

		return apperrors.NewValidationError(
			"plugins",
			fmt.Sprintf("declared plugin %q not found (not built-in and not found at %s)", rawDecl, pluginDir),
		)
	}
	return nil
}

// extractPluginName extracts the plugin name from a path or returns the input.
// Examples:
//   - "./plugins/file/file.wasm" -> "file"
//   - "file" -> "file"
//   - "/path/to/custom.wasm" -> "custom"
func extractPluginName(declared string) string {
	name := declared
	// If it's a path, extract the base name without extension
	if strings.Contains(name, "/") {
		base := filepath.Base(name)
		name = strings.TrimSuffix(base, ".wasm")
	}

	// Strip version/digest suffix if present (e.g. name@1.0 or name@sha256:...)
	if idx := strings.LastIndex(name, "@"); idx != -1 {
		name = name[:idx]
	}

	return name
}

// flattenCapabilities converts map of capabilities to flat list.
func flattenCapabilities(caps map[string][]capabilities.Capability) []capabilities.Capability {
	var flat []capabilities.Capability
	for _, pluginCaps := range caps {
		flat = append(flat, pluginCaps...)
	}
	return flat
}

// CheckFailed returns true if the execution result indicates failures.
func (uc *CheckProfileUseCase) CheckFailed(result *execution.ExecutionResult) bool {
	return result.Summary.FailedControls > 0 || result.Summary.ErrorControls > 0
}

// preparePluginEnvironment creates a temporary directory and populates it with
// symlinks to all required plugins (both local and OCI).
// Returns the path to the temp dir, a cleanup function, and any error.
func (uc *CheckProfileUseCase) preparePluginEnvironment(
	ctx context.Context,
	declaredPlugins []string,
	localPluginDir string,
) (string, func(), error) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "reglet-runtime-plugins-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp dir: %w", err)
	}

	cleanup := func() {
		_ = os.RemoveAll(tempDir)
	}

	for _, decl := range declaredPlugins {
		if err := uc.prepareSinglePlugin(ctx, decl, localPluginDir, tempDir); err != nil {
			cleanup()
			return "", nil, err
		}
	}

	return tempDir, cleanup, nil
}

func (uc *CheckProfileUseCase) prepareSinglePlugin(
	ctx context.Context,
	decl string,
	localPluginDir string,
	tempDir string,
) error {
	pluginName := extractPluginName(decl)
	var sourcePath string

	// 1. Try Local Source (Prioritize for tests/overrides)
	if filepath.IsAbs(decl) || strings.HasPrefix(decl, "./") || strings.HasPrefix(decl, "../") {
		sourcePath = decl
	} else if localPluginDir != "" {
		// Search in local plugin dir
		candidates := []string{
			filepath.Join(localPluginDir, pluginName, pluginName+".wasm"),
			filepath.Join(localPluginDir, pluginName+".wasm"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				sourcePath = c
				break
			}
		}
	}

	// 2. If locally found, use it (Link/Copy)
	if sourcePath != "" {
		// Resolve to absolute path to ensure valid symlink
		absSource, err := filepath.Abs(sourcePath)
		if err != nil {
			return fmt.Errorf("resolve abs path %s: %w", sourcePath, err)
		}
		sourcePath = absSource

		// Create subdirectory: tempDir/pluginName
		pluginDir := filepath.Join(tempDir, pluginName)
		if err := os.MkdirAll(pluginDir, 0o750); err != nil {
			return fmt.Errorf("create plugin dir %s: %w", pluginDir, err)
		}
		destPath := filepath.Join(pluginDir, pluginName+".wasm")

		// Always copy to avoid "path escapes from parent" errors in sandoxed runtimes
		data, err := os.ReadFile(filepath.Clean(sourcePath))
		if err != nil {
			return fmt.Errorf("read plugin %s: %w", sourcePath, err)
		}
		if err := os.WriteFile(destPath, data, 0o600); err != nil {
			return fmt.Errorf("write plugin to temp %s: %w", destPath, err)
		}
		return nil
	}

	// 3. Try OCI
	spec := &dto.PluginSpecDTO{Name: decl}
	if ref, err := spec.ToPluginReference(); err == nil && ref.Registry() != "" {
		uc.logger.Debug("resolving OCI plugin", "ref", decl)
		path, err := uc.pluginService.LoadPlugin(ctx, spec)
		if err != nil {
			return fmt.Errorf("load remote plugin %s: %w", decl, err)
		}

		pluginDir := filepath.Join(tempDir, pluginName)
		if err := os.MkdirAll(pluginDir, 0o750); err != nil {
			return fmt.Errorf("create plugin dir %s: %w", pluginDir, err)
		}
		destPath := filepath.Join(pluginDir, pluginName+".wasm")

		// Always copy to avoid sandbox issues
		data, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return fmt.Errorf("read cached plugin %s: %w", path, err)
		}
		if err := os.WriteFile(destPath, data, 0o600); err != nil {
			return fmt.Errorf("write plugin to temp %s: %w", destPath, err)
		}
		return nil
	}

	// 4. Built-in (Skip only if not found locally)
	if builtInPlugins[pluginName] {
		return nil
	}

	return apperrors.NewValidationError(
		"plugins",
		fmt.Sprintf("plugin %q not found locally or in registry, and is not built-in", decl),
	)
}
