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
		engineFactory:    engineFactory,
		logger:           logger,
	}
}

// Execute runs the complete check profile workflow.
func (uc *CheckProfileUseCase) Execute(ctx context.Context, req dto.CheckProfileRequest) (*dto.CheckProfileResponse, error) {
	startTime := time.Now()

	uc.logger.Info("loading profile", "path", req.ProfilePath)

	// 1. Load raw profile from YAML
	rawProfile, err := uc.profileLoader.LoadProfile(req.ProfilePath)
	if err != nil {
		return nil, apperrors.NewValidationError("profile", "failed to load profile", err.Error())
	}

	uc.logger.Info("profile loaded", "name", rawProfile.Metadata.Name, "version", rawProfile.Metadata.Version)

	// 2. Compile profile (apply defaults + validate)
	profile, err := uc.profileCompiler.Compile(rawProfile)
	if err != nil {
		return nil, apperrors.NewValidationError("profile", "compilation failed", err.Error())
	}

	uc.logger.Info("profile compiled and validated", "controls", profile.ControlCount())

	// 2b. Resolve versions and lock plugins (Phase 2.5)
	// We use the PROFILE directory for lockfile location
	lockfilePath := filepath.Join(filepath.Dir(req.ProfilePath), "reglet.lock")
	lockfile, err := uc.lockfileService.ResolvePlugins(ctx, profile.Profile, lockfilePath)
	if err != nil {
		return nil, apperrors.NewConfigurationError("lockfile", "failed to resolve plugins", err)
	}

	// Update profile with strict versions from lockfile
	// This ensures subsequent steps (verification, loading) use the pinned versions
	var strictPlugins []string
	for _, p := range profile.Plugins {
		spec, _ := entities.ParsePluginDeclaration(p) // Error checked in ResolvePlugins
		if locked := lockfile.GetPlugin(spec.Name); locked != nil {
			// Replace with strict version: "source@version"
			// If source has @digest, we preserve it?
			// Lockfile stores "Resolved" (e.g., "1.0.2") and "Source" (e.g. "reglet/aws")
			// We reconstruct the declaration.
			// Ideally we want "source@resolved"
			// But if Source is OCI path, it might be complicated.
			// For now, let's use the format that works for loader:
			// If we have "reglet/aws", resolved "1.0.2".
			// New decl: "reglet/aws@1.0.2"

			// We use locked.Source if it's correct?
			// locked.Source comes from spec.Source.

			// Let's assume standard behavior:
			// If locked.Resolved is "1.0.2", we append it.
			strictDecl := fmt.Sprintf("%s@%s", spec.Name, locked.Resolved)
			if spec.Name != spec.Source {
				// If alias used, maybe different?
				// For now simple append.
				// In real OCI world, we might need full reference.
				// Assuming simplified behavior for Phase 2.5 skeleton.
				strictDecl = fmt.Sprintf("%s@%s", spec.Source, locked.Resolved)
			}
			strictPlugins = append(strictPlugins, strictDecl)
		} else {
			strictPlugins = append(strictPlugins, p)
		}
	}
	profile.Plugins = strictPlugins

	uc.logger.Info("plugins resolved", "lockfile", lockfilePath)

	// 3. Apply filters to profile (validate filter references)
	if err := uc.validateFilters(profile, req.Filters); err != nil {
		return nil, err
	}

	// 4. Determine plugin directory
	pluginDir := req.Options.PluginDir
	if pluginDir == "" {
		pluginDir, err = uc.pluginResolver.ResolvePluginDir(ctx)
		if err != nil {
			uc.logger.Debug("failed to resolve plugin directory", "error", err)
			// Continue with empty plugin dir - engine will use embedded plugins
			pluginDir = ""
		}
	}

	// 5. Validate declared plugins exist and match used plugins
	if err := uc.validateDeclaredPlugins(profile, pluginDir); err != nil {
		return nil, err
	}

	// 6. Collect required capabilities using capability orchestrator
	requiredCaps, tempRuntime, err := uc.capOrchestrator.CollectCapabilities(ctx, profile, pluginDir)
	if err != nil {
		return nil, apperrors.NewConfigurationError("capabilities", "failed to collect capabilities", err)
	}
	defer func() {
		if tempRuntime != nil {
			_ = tempRuntime.Close(ctx)
		}
	}()

	// 7. Grant capabilities (interactive or auto-grant)
	grantedCaps, err := uc.capOrchestrator.GrantCapabilities(requiredCaps, req.Options.TrustPlugins)
	if err != nil {
		return nil, apperrors.NewCapabilityError("capability grant failed", flattenCapabilities(requiredCaps))
	}

	// 8. Create execution engine with granted capabilities and filters
	eng, err := uc.engineFactory.CreateEngine(ctx, profile, grantedCaps, pluginDir, req.Filters, req.Execution, req.Options.SkipSchemaValidation)
	if err != nil {
		return nil, apperrors.NewConfigurationError("engine", "failed to create engine", err)
	}
	defer func() {
		_ = eng.Close(ctx)
	}()

	// 9. Execute profile
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

	// 10. Build response
	response := &dto.CheckProfileResponse{
		ExecutionResult: result,
		Metadata: dto.ResponseMetadata{
			RequestID:   req.Metadata.RequestID,
			ProcessedAt: time.Now(),
			Duration:    time.Since(startTime),
		},
		Diagnostics: dto.Diagnostics{
			Capabilities: dto.CapabilityDiagnostics{
				Required: requiredCaps,
				Granted:  grantedCaps,
			},
		},
	}

	return response, nil
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

	// Build set of plugins used in observations
	usedPlugins := make(map[string]bool)
	for _, ctrl := range profile.GetAllControls() {
		for _, obs := range ctrl.ObservationDefinitions {
			usedPlugins[obs.Plugin] = true
		}
	}

	// Require plugins field if any observations use plugins
	if len(declaredPlugins) == 0 && len(usedPlugins) > 0 {
		var pluginList []string
		for p := range usedPlugins {
			pluginList = append(pluginList, p)
		}
		return apperrors.NewValidationError(
			"plugins",
			fmt.Sprintf("plugins field is required; add 'plugins:' section declaring: %v", pluginList),
		)
	}

	// Build set of declared plugin names for lookup
	declaredSet := make(map[string]bool)
	for _, declared := range declaredPlugins {
		declaredSet[extractPluginName(declared)] = true
	}

	// Validate each declared plugin exists
	for _, declared := range declaredPlugins {
		// Extract plugin name from path if it's a path (e.g., ./plugins/file/file.wasm -> file)
		pluginName := extractPluginName(declared)

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
		if strings.HasPrefix(declared, "./") || strings.HasPrefix(declared, "/") {
			if _, err := os.Stat(declared); err == nil {
				continue
			}
		}

		return apperrors.NewValidationError(
			"plugins",
			fmt.Sprintf("declared plugin %q not found (not built-in and not found at %s)", declared, pluginDir),
		)
	}

	// Error if plugins are used but not declared
	for used := range usedPlugins {
		if !declaredSet[used] {
			return apperrors.NewValidationError(
				"plugins",
				fmt.Sprintf("plugin %q used in observations but not declared in 'plugins:' section", used),
			)
		}
	}

	return nil
}

// extractPluginName extracts the plugin name from a path or returns the input.
// Examples:
//   - "./plugins/file/file.wasm" -> "file"
//   - "file" -> "file"
//   - "/path/to/custom.wasm" -> "custom"
func extractPluginName(declared string) string {
	// If it's a path, extract the base name without extension
	if strings.Contains(declared, "/") {
		base := filepath.Base(declared)
		return strings.TrimSuffix(base, ".wasm")
	}
	return declared
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
