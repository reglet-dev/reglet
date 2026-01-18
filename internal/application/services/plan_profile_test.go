package services

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/reglet-dev/reglet/internal/application/dto"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProfileLoaderForPlan implements ports.ProfileLoader for testing.
type mockProfileLoaderForPlan struct {
	profile *entities.Profile
	err     error
}

func (m *mockProfileLoaderForPlan) LoadProfile(_ string) (*entities.Profile, error) {
	return m.profile, m.err
}

func (m *mockProfileLoaderForPlan) LoadProfileWithCLIVars(_ string, _ map[string]interface{}) (*entities.Profile, error) {
	return m.profile, m.err
}

// newTestProfileCompiler creates a ProfileCompiler for tests.
func newTestProfileCompiler() *services.ProfileCompiler {
	return services.NewProfileCompiler()
}

func TestNewPlanProfileUseCase_RequiredDependencies(t *testing.T) {
	t.Parallel()

	loader := &mockProfileLoaderForPlan{}
	compiler := newTestProfileCompiler()

	uc := NewPlanProfileUseCase(loader, compiler)

	require.NotNil(t, uc)
	assert.NotNil(t, uc.profileLoader)
	assert.NotNil(t, uc.profileCompiler)
	assert.NotNil(t, uc.depResolver)
	assert.NotNil(t, uc.logger)
}

func TestNewPlanProfileUseCase_WithOptions(t *testing.T) {
	t.Parallel()

	loader := &mockProfileLoaderForPlan{}
	compiler := newTestProfileCompiler()
	customLogger := slog.Default()

	uc := NewPlanProfileUseCase(
		loader,
		compiler,
		WithPlanLogger(customLogger),
	)

	require.NotNil(t, uc)
	assert.Equal(t, customLogger, uc.logger)
}

func TestPlanProfileUseCase_Execute_Success(t *testing.T) {
	t.Parallel()

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "test-profile",
			Version: "1.0.0",
		},
		Plugins: []string{"file"},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "ctrl-1",
					Name: "Control One",
					ObservationDefinitions: []entities.ObservationDefinition{
						{Plugin: "file", Config: map[string]interface{}{"path": "/etc/test"}},
					},
				},
				{
					ID:        "ctrl-2",
					Name:      "Control Two",
					DependsOn: []string{"ctrl-1"},
					ObservationDefinitions: []entities.ObservationDefinition{
						{Plugin: "file", Config: map[string]interface{}{"path": "/etc/other"}},
					},
				},
			},
		},
	}

	loader := &mockProfileLoaderForPlan{profile: profile}
	compiler := newTestProfileCompiler()

	uc := NewPlanProfileUseCase(loader, compiler)

	req := dto.PlanProfileRequest{
		ProfilePath: "test-profile.yaml",
		Metadata:    dto.RequestMetadata{RequestID: "test-123"},
	}

	resp, err := uc.Execute(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Plan)

	assert.Equal(t, "test-profile", resp.Plan.ProfileName)
	assert.Equal(t, "1.0.0", resp.Plan.ProfileVersion)
	assert.Equal(t, 2, resp.Plan.TotalControls)
	assert.Equal(t, 2, resp.Plan.LevelCount())
	assert.True(t, resp.Plan.HasDependencies)
	assert.Equal(t, "test-123", resp.Metadata.RequestID)
}

func TestPlanProfileUseCase_Execute_WithFilters(t *testing.T) {
	t.Parallel()

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "filtered-profile",
			Version: "2.0.0",
		},
		Plugins: []string{"file"},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:       "high-1",
					Name:     "High Severity Check",
					Severity: "high",
					ObservationDefinitions: []entities.ObservationDefinition{
						{Plugin: "file"},
					},
				},
				{
					ID:       "low-1",
					Name:     "Low Severity Check",
					Severity: "low",
					ObservationDefinitions: []entities.ObservationDefinition{
						{Plugin: "file"},
					},
				},
				{
					ID:       "critical-1",
					Name:     "Critical Check",
					Severity: "critical",
					ObservationDefinitions: []entities.ObservationDefinition{
						{Plugin: "file"},
					},
				},
			},
		},
	}

	loader := &mockProfileLoaderForPlan{profile: profile}
	compiler := newTestProfileCompiler()

	uc := NewPlanProfileUseCase(loader, compiler)

	req := dto.PlanProfileRequest{
		ProfilePath: "test-profile.yaml",
		Filters: dto.FilterOptions{
			IncludeSeverities: []string{"high", "critical"},
		},
	}

	resp, err := uc.Execute(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp.Plan)

	// Only high and critical controls should be included
	assert.Equal(t, 2, resp.Plan.TotalControls)

	// All should be in level 0 (no dependencies)
	assert.Equal(t, 1, resp.Plan.LevelCount())
}

func TestPlanProfileUseCase_Execute_LoadError(t *testing.T) {
	t.Parallel()

	loader := &mockProfileLoaderForPlan{err: errors.New("file not found")}
	compiler := newTestProfileCompiler()

	uc := NewPlanProfileUseCase(loader, compiler)

	req := dto.PlanProfileRequest{
		ProfilePath: "nonexistent.yaml",
	}

	resp, err := uc.Execute(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to load profile")
}

func TestPlanProfileUseCase_Execute_EmptyAfterFilter(t *testing.T) {
	t.Parallel()

	// Valid profile with one control, but filtering will exclude it
	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "empty-after-filter",
			Version: "1.0.0",
		},
		Plugins: []string{"file"},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:       "excluded-1",
					Name:     "Excluded Control",
					Severity: "low",
					Tags:     []string{"slow"},
					ObservationDefinitions: []entities.ObservationDefinition{
						{Plugin: "file"},
					},
				},
			},
		},
	}

	loader := &mockProfileLoaderForPlan{profile: profile}
	compiler := newTestProfileCompiler()

	uc := NewPlanProfileUseCase(loader, compiler)

	req := dto.PlanProfileRequest{
		ProfilePath: "empty-after-filter.yaml",
		Filters: dto.FilterOptions{
			ExcludeTags: []string{"slow"}, // Exclude all controls with 'slow' tag
		},
	}

	resp, err := uc.Execute(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp.Plan)
	assert.True(t, resp.Plan.IsEmpty())
	assert.Equal(t, 0, resp.Plan.TotalControls)
}

func TestPlanProfileUseCase_Execute_FilterExcludesAll(t *testing.T) {
	t.Parallel()

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "test-profile",
			Version: "1.0.0",
		},
		Plugins: []string{"file"},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:       "low-1",
					Name:     "Low Check",
					Severity: "low",
					ObservationDefinitions: []entities.ObservationDefinition{
						{Plugin: "file"},
					},
				},
			},
		},
	}

	loader := &mockProfileLoaderForPlan{profile: profile}
	compiler := newTestProfileCompiler()

	uc := NewPlanProfileUseCase(loader, compiler)

	req := dto.PlanProfileRequest{
		ProfilePath: "test.yaml",
		Filters: dto.FilterOptions{
			IncludeSeverities: []string{"critical"},
		},
	}

	resp, err := uc.Execute(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp.Plan)
	assert.True(t, resp.Plan.IsEmpty())
}

func TestHasFilters(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		filters  dto.FilterOptions
		expected bool
	}{
		{"empty", dto.FilterOptions{}, false},
		{"includeTags", dto.FilterOptions{IncludeTags: []string{"test"}}, true},
		{"includeSeverities", dto.FilterOptions{IncludeSeverities: []string{"high"}}, true},
		{"includeControlIDs", dto.FilterOptions{IncludeControlIDs: []string{"id"}}, true},
		{"excludeTags", dto.FilterOptions{ExcludeTags: []string{"slow"}}, true},
		{"excludeControlIDs", dto.FilterOptions{ExcludeControlIDs: []string{"id"}}, true},
		{"filterExpression", dto.FilterOptions{FilterExpression: "severity == 'high'"}, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := hasFilters(tc.filters)
			assert.Equal(t, tc.expected, result)
		})
	}
}
