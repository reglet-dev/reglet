// Package services contains application use cases.
package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/reglet-dev/reglet/internal/application/dto"
	apperrors "github.com/reglet-dev/reglet/internal/application/errors"
	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	domainservices "github.com/reglet-dev/reglet/internal/domain/services"
)

// ValidateProfileUseCase validates profile structure without execution.
// This provides fast feedback during profile development by checking:
// - Profile metadata (name, version)
// - Control definitions (ID, name, observations)
// - Dependency graph (cycle detection)
// - Expect expression syntax (expr-lang)
type ValidateProfileUseCase struct {
	profileLoader    ports.ProfileLoader
	profileValidator ports.ProfileValidator
	profileCompiler  *domainservices.ProfileCompiler
	depResolver      *domainservices.DependencyResolver
	expectValidator  *domainservices.ExpectValidator
	logger           *slog.Logger
}

// ValidateProfileUseCaseOption configures a ValidateProfileUseCase.
type ValidateProfileUseCaseOption func(*ValidateProfileUseCase)

// WithValidateProfileValidator sets the profile validator.
func WithValidateProfileValidator(v ports.ProfileValidator) ValidateProfileUseCaseOption {
	return func(uc *ValidateProfileUseCase) { uc.profileValidator = v }
}

// WithValidateLogger sets the logger.
func WithValidateLogger(l *slog.Logger) ValidateProfileUseCaseOption {
	return func(uc *ValidateProfileUseCase) { uc.logger = l }
}

// WithValidateDependencyResolver sets a custom dependency resolver.
func WithValidateDependencyResolver(r *domainservices.DependencyResolver) ValidateProfileUseCaseOption {
	return func(uc *ValidateProfileUseCase) { uc.depResolver = r }
}

// WithValidateExpectValidator sets a custom expect validator.
func WithValidateExpectValidator(v *domainservices.ExpectValidator) ValidateProfileUseCaseOption {
	return func(uc *ValidateProfileUseCase) { uc.expectValidator = v }
}

// NewValidateProfileUseCase creates a new validate profile use case.
// ProfileLoader and ProfileCompiler are required dependencies.
func NewValidateProfileUseCase(
	profileLoader ports.ProfileLoader,
	profileCompiler *domainservices.ProfileCompiler,
	opts ...ValidateProfileUseCaseOption,
) *ValidateProfileUseCase {
	uc := &ValidateProfileUseCase{
		profileLoader:   profileLoader,
		profileCompiler: profileCompiler,
		depResolver:     domainservices.NewDependencyResolver(),
		expectValidator: domainservices.NewExpectValidator(),
		logger:          slog.Default(),
	}
	for _, opt := range opts {
		opt(uc)
	}
	return uc
}

// Execute validates the profile and returns validation results.
func (uc *ValidateProfileUseCase) Execute(
	ctx context.Context,
	req dto.ValidateProfileRequest,
) (*dto.ValidateProfileResponse, error) {
	startTime := time.Now()

	uc.logger.Info("validating profile", "path", req.ProfilePath)

	var validationErrors []dto.ValidationError
	var warnings []string

	// 1. Load profile
	profile, err := uc.profileLoader.LoadProfile(req.ProfilePath)
	if err != nil {
		return nil, apperrors.NewValidationError("profile", "failed to load profile", err.Error())
	}

	uc.logger.Debug("profile loaded", "name", profile.Metadata.Name, "version", profile.Metadata.Version)

	// 2. Structural validation
	if uc.profileValidator != nil {
		if err := uc.profileValidator.Validate(profile); err != nil {
			validationErrors = append(validationErrors, dto.ValidationError{
				Type:    "structural",
				Path:    "profile",
				Message: err.Error(),
			})
		}
	}

	// 3. Compile profile (applies defaults, validates)
	compiledProfile, err := uc.profileCompiler.Compile(profile)
	if err != nil {
		validationErrors = append(validationErrors, dto.ValidationError{
			Type:    "structural",
			Path:    "profile",
			Message: fmt.Sprintf("compilation failed: %s", err.Error()),
		})
		// Return early on compilation failure - can't proceed with further validation
		return uc.buildResponse(profile, validationErrors, warnings, startTime, req.Metadata.RequestID), nil
	}

	// 4. Validate dependency graph (cycle detection)
	controls := compiledProfile.GetAllControls()
	_, err = uc.depResolver.BuildControlDAG(controls)
	if err != nil {
		validationErrors = append(validationErrors, dto.ValidationError{
			Type:    "dependency",
			Path:    "controls",
			Message: err.Error(),
		})
	}

	// 5. Validate expect expressions (unless skipped)
	if !req.SkipExpectValidation {
		expectErrors := uc.validateExpects(controls)
		validationErrors = append(validationErrors, expectErrors...)
	}

	// 6. Collect statistics
	stats := uc.collectStats(controls)

	// 7. Build response
	response := uc.buildResponse(profile, validationErrors, warnings, startTime, req.Metadata.RequestID)
	response.Stats = stats

	uc.logger.Info("profile validation complete",
		"valid", response.Valid,
		"errors", len(response.Errors),
		"controls", stats.ControlCount)

	return response, nil
}

// validateExpects validates all expect expressions in the profile.
func (uc *ValidateProfileUseCase) validateExpects(controls []entities.Control) []dto.ValidationError {
	var errors []dto.ValidationError

	for ctrlIdx, ctrl := range controls {
		for obsIdx, obs := range ctrl.ObservationDefinitions {
			expectErrors := uc.expectValidator.ValidateObservationExpects(obs.Expect)
			for expIdx, expErr := range expectErrors {
				errors = append(errors, dto.ValidationError{
					Type:    "expect",
					Path:    fmt.Sprintf("controls[%d].observations[%d].expect[%d]", ctrlIdx, obsIdx, expIdx),
					Message: fmt.Sprintf("control %s: %s", ctrl.ID, expErr.Message),
				})
			}
		}
	}

	return errors
}

// collectStats gathers profile statistics.
func (uc *ValidateProfileUseCase) collectStats(controls []entities.Control) dto.ValidationStats {
	stats := dto.ValidationStats{
		ControlCount: len(controls),
	}

	pluginSet := make(map[string]bool)

	for _, ctrl := range controls {
		stats.ObservationCount += len(ctrl.ObservationDefinitions)
		for _, obs := range ctrl.ObservationDefinitions {
			stats.ExpectCount += len(obs.Expect)
			pluginSet[obs.Plugin] = true
		}
	}

	for plugin := range pluginSet {
		stats.PluginsUsed = append(stats.PluginsUsed, plugin)
	}

	return stats
}

// buildResponse constructs the final response.
func (uc *ValidateProfileUseCase) buildResponse(
	profile *entities.Profile,
	errors []dto.ValidationError,
	warnings []string,
	startTime time.Time,
	requestID string,
) *dto.ValidateProfileResponse {
	return &dto.ValidateProfileResponse{
		Valid:       len(errors) == 0,
		ProfileName: profile.Metadata.Name,
		Version:     profile.Metadata.Version,
		Errors:      errors,
		Warnings:    warnings,
		Metadata: dto.ResponseMetadata{
			RequestID:   requestID,
			ProcessedAt: time.Now(),
			Duration:    time.Since(startTime),
		},
	}
}
