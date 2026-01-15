// Package services contains application use cases.
package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/expr-lang/expr"
	"github.com/reglet-dev/reglet/internal/application/dto"
	apperrors "github.com/reglet-dev/reglet/internal/application/errors"
	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/services"
)

// PlanProfileUseCase generates an execution plan without running controls.
// This provides a dry-run view showing what would execute and in what order.
type PlanProfileUseCase struct {
	profileLoader   ports.ProfileLoader
	profileCompiler *services.ProfileCompiler
	depResolver     *services.DependencyResolver
	logger          *slog.Logger
}

// PlanProfileUseCaseOption configures a PlanProfileUseCase.
type PlanProfileUseCaseOption func(*PlanProfileUseCase)

// NewPlanProfileUseCase creates a new plan profile use case.
// ProfileLoader and ProfileCompiler are required dependencies.
func NewPlanProfileUseCase(
	profileLoader ports.ProfileLoader,
	profileCompiler *services.ProfileCompiler,
	opts ...PlanProfileUseCaseOption,
) *PlanProfileUseCase {
	uc := &PlanProfileUseCase{
		profileLoader:   profileLoader,
		profileCompiler: profileCompiler,
		depResolver:     services.NewDependencyResolver(),
		logger:          slog.Default(),
	}
	for _, opt := range opts {
		opt(uc)
	}
	return uc
}

// WithPlanDependencyResolver sets a custom dependency resolver.
func WithPlanDependencyResolver(r *services.DependencyResolver) PlanProfileUseCaseOption {
	return func(uc *PlanProfileUseCase) { uc.depResolver = r }
}

// WithPlanLogger sets the logger.
func WithPlanLogger(l *slog.Logger) PlanProfileUseCaseOption {
	return func(uc *PlanProfileUseCase) { uc.logger = l }
}

// Execute generates the execution plan.
func (uc *PlanProfileUseCase) Execute(
	ctx context.Context,
	req dto.PlanProfileRequest,
) (*dto.PlanProfileResponse, error) {
	startTime := time.Now()

	uc.logger.Info("planning profile execution", "path", req.ProfilePath)

	// 1. Load profile
	rawProfile, err := uc.profileLoader.LoadProfile(req.ProfilePath)
	if err != nil {
		return nil, apperrors.NewValidationError("profile", "failed to load profile", err.Error())
	}

	uc.logger.Debug("profile loaded", "name", rawProfile.Metadata.Name, "version", rawProfile.Metadata.Version)

	// 2. Compile profile
	profile, err := uc.profileCompiler.Compile(rawProfile)
	if err != nil {
		return nil, apperrors.NewValidationError("profile", "compilation failed", err.Error())
	}

	// 3. Get controls and apply filters
	controls := profile.GetAllControls()
	if hasFilters(req.Filters) {
		var filterErr error
		controls, filterErr = uc.applyFilters(controls, req.Filters)
		if filterErr != nil {
			return nil, filterErr
		}
	}

	uc.logger.Debug("controls selected", "count", len(controls))

	// 4. Build dependency DAG
	levels, err := uc.depResolver.BuildControlDAG(controls)
	if err != nil {
		return nil, apperrors.NewValidationError("dependency", "failed to build execution plan", err.Error())
	}

	// 5. Convert to ExecutionPlan
	plan := uc.buildExecutionPlan(profile, levels)

	uc.logger.Info("execution plan generated",
		"controls", plan.TotalControls,
		"levels", plan.LevelCount(),
		"parallelism", plan.MaxParallelism)

	// 6. Return response
	return &dto.PlanProfileResponse{
		Plan: plan,
		Metadata: dto.ResponseMetadata{
			RequestID:   req.Metadata.RequestID,
			ProcessedAt: time.Now(),
			Duration:    time.Since(startTime),
		},
	}, nil
}

// applyFilters applies the filter options to select controls.
func (uc *PlanProfileUseCase) applyFilters(
	controls []entities.Control,
	filters dto.FilterOptions,
) ([]entities.Control, error) {
	filter := services.NewControlFilter()

	// Exclusive control selection
	if len(filters.IncludeControlIDs) > 0 {
		filter.WithExclusiveControls(filters.IncludeControlIDs)
	}

	// Exclusion filters
	if len(filters.ExcludeControlIDs) > 0 {
		filter.WithExcludedControls(filters.ExcludeControlIDs)
	}
	if len(filters.ExcludeTags) > 0 {
		filter.WithExcludedTags(filters.ExcludeTags)
	}

	// Inclusion filters
	if len(filters.IncludeTags) > 0 {
		filter.WithIncludedTags(filters.IncludeTags)
	}
	if len(filters.IncludeSeverities) > 0 {
		filter.WithIncludedSeverities(filters.IncludeSeverities)
	}

	// Advanced filter expression
	if filters.FilterExpression != "" {
		program, err := expr.Compile(filters.FilterExpression,
			expr.Env(services.ControlEnv{}),
			expr.AsBool())
		if err != nil {
			return nil, apperrors.NewValidationError(
				"filters",
				fmt.Sprintf("invalid filter expression: %v", err),
			)
		}
		filter.WithFilterExpression(program)
	}

	// Apply filter to controls
	var result []entities.Control
	for _, ctrl := range controls {
		if shouldRun, _ := filter.ShouldRun(ctrl); shouldRun {
			result = append(result, ctrl)
		}
	}

	return result, nil
}

// buildExecutionPlan converts domain ControlLevels to an ExecutionPlan.
func (uc *PlanProfileUseCase) buildExecutionPlan(
	profile *entities.ValidatedProfile,
	levels []services.ControlLevel,
) *entities.ExecutionPlan {
	planLevels := make([]entities.ExecutionPlanLevel, len(levels))

	for i, level := range levels {
		summaries := make([]entities.ControlSummary, len(level.Controls))
		for j, ctrl := range level.Controls {
			summaries[j] = entities.ControlSummaryFromControl(ctrl)
		}
		planLevels[i] = entities.ExecutionPlanLevel{
			Level:    level.Level,
			Controls: summaries,
		}
	}

	meta := profile.GetMetadata()
	return entities.NewExecutionPlan(meta.Name, meta.Version, planLevels)
}

// hasFilters returns true if any filter options are set.
func hasFilters(f dto.FilterOptions) bool {
	return len(f.IncludeTags) > 0 ||
		len(f.IncludeSeverities) > 0 ||
		len(f.IncludeControlIDs) > 0 ||
		len(f.ExcludeTags) > 0 ||
		len(f.ExcludeControlIDs) > 0 ||
		f.FilterExpression != ""
}
