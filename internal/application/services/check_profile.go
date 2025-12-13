// Package services contains application use cases.
package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/whiskeyjimbo/reglet/internal/application/dto"
	apperrors "github.com/whiskeyjimbo/reglet/internal/application/errors"
	"github.com/whiskeyjimbo/reglet/internal/application/ports"
	"github.com/whiskeyjimbo/reglet/internal/domain/capabilities"
	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/wasm"
)

// CheckProfileUseCase orchestrates the complete profile check workflow.
type CheckProfileUseCase struct {
	profileLoader       ports.ProfileLoader
	profileValidator    ports.ProfileValidator
	configProvider      ports.SystemConfigProvider
	pluginResolver      ports.PluginDirectoryResolver
	capabilityCollector ports.CapabilityCollector
	capabilityGranter   ports.CapabilityGranter
	engineFactory       ports.EngineFactory
	logger              *slog.Logger
}

// NewCheckProfileUseCase creates a new check profile use case.
func NewCheckProfileUseCase(
	profileLoader ports.ProfileLoader,
	profileValidator ports.ProfileValidator,
	configProvider ports.SystemConfigProvider,
	pluginResolver ports.PluginDirectoryResolver,
	capCollector ports.CapabilityCollector,
	capGranter ports.CapabilityGranter,
	engineFactory ports.EngineFactory,
	logger *slog.Logger,
) *CheckProfileUseCase {
	if logger == nil {
		logger = slog.Default()
	}

	return &CheckProfileUseCase{
		profileLoader:       profileLoader,
		profileValidator:    profileValidator,
		configProvider:      configProvider,
		pluginResolver:      pluginResolver,
		capabilityCollector: capCollector,
		capabilityGranter:   capGranter,
		engineFactory:       engineFactory,
		logger:              logger,
	}
}

// Execute runs the complete check profile workflow.
func (uc *CheckProfileUseCase) Execute(ctx context.Context, req dto.CheckProfileRequest) (*dto.CheckProfileResponse, error) {
	startTime := time.Now()

	uc.logger.Info("starting profile check",
		"profile", req.ProfilePath,
		"request_id", req.Metadata.RequestID)

	// 1. Load and validate profile
	profile, err := uc.loadAndValidateProfile(ctx, req.ProfilePath)
	if err != nil {
		return nil, err
	}

	// 2. Apply filters to profile
	if err := uc.applyFilters(profile, req.Filters); err != nil {
		return nil, err
	}

	// 3. Resolve plugin directory
	pluginDir := req.Options.PluginDir
	if pluginDir == "" {
		pluginDir, err = uc.pluginResolver.ResolvePluginDir(ctx)
		if err != nil {
			return nil, apperrors.NewConfigurationError("plugin_dir", "failed to resolve plugin directory", err)
		}
	}

	// 4. Create temporary runtime for capability collection
	tempRuntime, err := uc.createTemporaryRuntime(ctx)
	if err != nil {
		return nil, err
	}
	defer tempRuntime.Close(ctx)

	// 5. Collect required capabilities
	requiredCaps, err := uc.capabilityCollector.CollectRequiredCapabilities(ctx, profile, tempRuntime, pluginDir)
	if err != nil {
		return nil, apperrors.NewConfigurationError("capabilities", "failed to collect capabilities", err)
	}

	// 6. Grant capabilities
	grantedCaps, err := uc.capabilityGranter.GrantCapabilities(ctx, requiredCaps, req.Options.TrustPlugins)
	if err != nil {
		return nil, apperrors.NewCapabilityError("capability grant failed", flattenCapabilities(requiredCaps))
	}

	// 7. Create execution engine with granted capabilities
	execEngine, err := uc.engineFactory.CreateEngine(ctx, profile, grantedCaps, pluginDir, req.Options.SkipSchemaValidation)
	if err != nil {
		return nil, apperrors.NewConfigurationError("engine", "failed to create execution engine", err)
	}
	defer execEngine.Close(ctx)

	// 8. Execute profile
	result, err := execEngine.Execute(ctx, profile)
	if err != nil {
		return nil, apperrors.NewExecutionError("", "profile execution failed", err)
	}

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

	uc.logger.Info("profile check complete",
		"profile", req.ProfilePath,
		"request_id", req.Metadata.RequestID,
		"duration", response.Metadata.Duration,
		"passed", result.Summary.PassedControls,
		"failed", result.Summary.FailedControls)

	return response, nil
}

// loadAndValidateProfile loads and validates the profile.
func (uc *CheckProfileUseCase) loadAndValidateProfile(ctx context.Context, profilePath string) (*entities.Profile, error) {
	// Load profile
	profile, err := uc.profileLoader.LoadProfile(profilePath)
	if err != nil {
		return nil, apperrors.NewValidationError("profile", "failed to load profile", err.Error())
	}

	// Validate structure
	if err := uc.profileValidator.Validate(profile); err != nil {
		return nil, apperrors.NewValidationError("profile", "validation failed", err.Error())
	}

	return profile, nil
}

// applyFilters applies execution filters to the profile.
func (uc *CheckProfileUseCase) applyFilters(profile *entities.Profile, filters dto.FilterOptions) error {
	// Validate filter references
	if len(filters.IncludeControlIDs) > 0 {
		for _, id := range filters.IncludeControlIDs {
			if !profile.HasControl(id) {
				return apperrors.NewValidationError(
					"filters",
					fmt.Sprintf("control %s not found in profile", id),
				)
			}
		}
	}

	// Filter expression validation would happen here
	// For now, we'll let the engine handle it

	return nil
}

// createTemporaryRuntime creates a temporary WASM runtime for capability collection.
func (uc *CheckProfileUseCase) createTemporaryRuntime(ctx context.Context) (*wasm.Runtime, error) {
	runtime, err := wasm.NewRuntime(ctx)
	if err != nil {
		return nil, apperrors.NewConfigurationError("wasm", "failed to create temporary runtime", err)
	}
	return runtime, nil
}

// flattenCapabilities converts map of capabilities to flat list.
func flattenCapabilities(caps map[string][]capabilities.Capability) []capabilities.Capability {
	var flat []capabilities.Capability
	for _, pluginCaps := range caps {
		flat = append(flat, pluginCaps...)
	}
	return flat
}
