// Package services contains application use cases.
package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/expr-lang/expr"
	"github.com/whiskeyjimbo/reglet/internal/application/dto"
	apperrors "github.com/whiskeyjimbo/reglet/internal/application/errors"
	"github.com/whiskeyjimbo/reglet/internal/application/ports"
	"github.com/whiskeyjimbo/reglet/internal/domain/capabilities"
	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
	"github.com/whiskeyjimbo/reglet/internal/domain/execution"
	"github.com/whiskeyjimbo/reglet/internal/domain/services"
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

	// 5. Collect required capabilities using capability orchestrator
	// Note: ValidatedProfile embeds *Profile, so we can access it directly
	requiredCaps, tempRuntime, err := uc.capOrchestrator.CollectCapabilities(ctx, profile.Profile, pluginDir)
	if err != nil {
		return nil, apperrors.NewConfigurationError("capabilities", "failed to collect capabilities", err)
	}
	defer func() {
		if tempRuntime != nil {
			_ = tempRuntime.Close(ctx)
		}
	}()

	// 6. Grant capabilities (interactive or auto-grant)
	grantedCaps, err := uc.capOrchestrator.GrantCapabilities(requiredCaps, req.Options.TrustPlugins)
	if err != nil {
		return nil, apperrors.NewCapabilityError("capability grant failed", flattenCapabilities(requiredCaps))
	}

	// 7. Create execution engine with granted capabilities
	eng, err := uc.engineFactory.CreateEngine(ctx, profile.Profile, grantedCaps, pluginDir, req.Options.SkipSchemaValidation)
	if err != nil {
		return nil, apperrors.NewConfigurationError("engine", "failed to create engine", err)
	}
	defer func() {
		_ = eng.Close(ctx)
	}()

	// 8. Execute profile
	uc.logger.Info("executing profile")
	result, err := eng.Execute(ctx, profile.Profile)
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

	// 9. Build response
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
