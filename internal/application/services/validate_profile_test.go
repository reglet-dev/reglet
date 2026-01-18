package services

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/reglet-dev/reglet/internal/application/dto"
	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	domainservices "github.com/reglet-dev/reglet/internal/domain/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProfileLoaderForValidate implements ports.ProfileLoader for testing.
type mockProfileLoaderForValidate struct {
	profile *entities.Profile
	err     error
}

func (m *mockProfileLoaderForValidate) LoadProfile(_ string) (*entities.Profile, error) {
	return m.profile, m.err
}

func (m *mockProfileLoaderForValidate) LoadProfileWithCLIVars(_ string, _ map[string]interface{}) (*entities.Profile, error) {
	return m.profile, m.err
}

// mockProfileValidatorForValidate implements ports.ProfileValidator for testing.
type mockProfileValidatorForValidate struct {
	err error
}

func (m *mockProfileValidatorForValidate) Validate(_ *entities.Profile) error {
	return m.err
}

func (m *mockProfileValidatorForValidate) ValidateWithSchemas(_ context.Context, _ *entities.Profile, _ ports.PluginRuntime) error {
	return nil
}

func TestNewValidateProfileUseCase_RequiredDependencies(t *testing.T) {
	t.Parallel()

	loader := &mockProfileLoaderForValidate{}
	compiler := domainservices.NewProfileCompiler()

	uc := NewValidateProfileUseCase(loader, compiler)

	require.NotNil(t, uc)
	assert.NotNil(t, uc.profileLoader)
	assert.NotNil(t, uc.profileCompiler)
	assert.NotNil(t, uc.depResolver)
	assert.NotNil(t, uc.expectValidator)
	assert.NotNil(t, uc.logger)
}

func TestNewValidateProfileUseCase_WithOptions(t *testing.T) {
	t.Parallel()

	loader := &mockProfileLoaderForValidate{}
	compiler := domainservices.NewProfileCompiler()
	customLogger := slog.Default()
	customValidator := &mockProfileValidatorForValidate{}

	uc := NewValidateProfileUseCase(
		loader,
		compiler,
		WithValidateLogger(customLogger),
		WithValidateProfileValidator(customValidator),
	)

	require.NotNil(t, uc)
	assert.Equal(t, customLogger, uc.logger)
	assert.Equal(t, customValidator, uc.profileValidator)
}

func TestValidateProfileUseCase_Execute_ValidProfile(t *testing.T) {
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
						{
							Plugin: "file",
							Config: map[string]interface{}{"path": "/etc/test"},
							Expect: []string{"data.exists == true"},
						},
					},
				},
			},
		},
	}

	loader := &mockProfileLoaderForValidate{profile: profile}
	compiler := domainservices.NewProfileCompiler()

	uc := NewValidateProfileUseCase(loader, compiler)

	req := dto.ValidateProfileRequest{
		ProfilePath: "test-profile.yaml",
		Metadata:    dto.RequestMetadata{RequestID: "test-123"},
	}

	resp, err := uc.Execute(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Valid)
	assert.Equal(t, "test-profile", resp.ProfileName)
	assert.Equal(t, "1.0.0", resp.Version)
	assert.Empty(t, resp.Errors)
	assert.Equal(t, 1, resp.Stats.ControlCount)
	assert.Equal(t, 1, resp.Stats.ObservationCount)
	assert.Equal(t, 1, resp.Stats.ExpectCount)
	assert.Contains(t, resp.Stats.PluginsUsed, "file")
	assert.Equal(t, "test-123", resp.Metadata.RequestID)
}

func TestValidateProfileUseCase_Execute_LoadError(t *testing.T) {
	t.Parallel()

	loader := &mockProfileLoaderForValidate{err: errors.New("file not found")}
	compiler := domainservices.NewProfileCompiler()

	uc := NewValidateProfileUseCase(loader, compiler)

	req := dto.ValidateProfileRequest{
		ProfilePath: "nonexistent.yaml",
	}

	resp, err := uc.Execute(context.Background(), req)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to load profile")
}

func TestValidateProfileUseCase_Execute_StructuralError(t *testing.T) {
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
						{Plugin: "file", Config: map[string]interface{}{}},
					},
				},
			},
		},
	}

	loader := &mockProfileLoaderForValidate{profile: profile}
	compiler := domainservices.NewProfileCompiler()
	validator := &mockProfileValidatorForValidate{err: errors.New("invalid profile structure")}

	uc := NewValidateProfileUseCase(
		loader,
		compiler,
		WithValidateProfileValidator(validator),
	)

	req := dto.ValidateProfileRequest{
		ProfilePath: "test.yaml",
	}

	resp, err := uc.Execute(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Valid)
	require.Len(t, resp.Errors, 1)
	assert.Equal(t, "structural", resp.Errors[0].Type)
}

func TestValidateProfileUseCase_Execute_DependencyCycle(t *testing.T) {
	t.Parallel()

	// Create profile with circular dependency
	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "cycle-profile",
			Version: "1.0.0",
		},
		Plugins: []string{"file"},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:        "ctrl-a",
					Name:      "Control A",
					DependsOn: []string{"ctrl-b"},
					ObservationDefinitions: []entities.ObservationDefinition{
						{Plugin: "file", Config: map[string]interface{}{}},
					},
				},
				{
					ID:        "ctrl-b",
					Name:      "Control B",
					DependsOn: []string{"ctrl-a"},
					ObservationDefinitions: []entities.ObservationDefinition{
						{Plugin: "file", Config: map[string]interface{}{}},
					},
				},
			},
		},
	}

	loader := &mockProfileLoaderForValidate{profile: profile}
	compiler := domainservices.NewProfileCompiler()

	uc := NewValidateProfileUseCase(loader, compiler)

	req := dto.ValidateProfileRequest{
		ProfilePath: "cycle.yaml",
	}

	resp, err := uc.Execute(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Valid)

	// Cycle may be detected during compilation (structural) or DAG building (dependency)
	hasCircularError := false
	for _, e := range resp.Errors {
		if strings.Contains(e.Message, "circular") {
			hasCircularError = true
		}
	}
	assert.True(t, hasCircularError, "expected circular dependency error")
}

func TestValidateProfileUseCase_Execute_InvalidExpects(t *testing.T) {
	t.Parallel()

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "expect-profile",
			Version: "1.0.0",
		},
		Plugins: []string{"file"},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "ctrl-1",
					Name: "Control One",
					ObservationDefinitions: []entities.ObservationDefinition{
						{
							Plugin: "file",
							Config: map[string]interface{}{},
							Expect: []string{
								"data.exists == true", // valid
								"(invalid syntax",     // invalid - unclosed paren
							},
						},
					},
				},
			},
		},
	}

	loader := &mockProfileLoaderForValidate{profile: profile}
	compiler := domainservices.NewProfileCompiler()

	uc := NewValidateProfileUseCase(loader, compiler)

	req := dto.ValidateProfileRequest{
		ProfilePath: "expect.yaml",
	}

	resp, err := uc.Execute(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Valid)

	// Should have exactly 1 expect error (the unclosed paren)
	expectErrors := 0
	for _, e := range resp.Errors {
		if e.Type == "expect" {
			expectErrors++
			assert.Contains(t, e.Path, "expect")
		}
	}
	assert.Equal(t, 1, expectErrors)
}

func TestValidateProfileUseCase_Execute_SkipExpectValidation(t *testing.T) {
	t.Parallel()

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "skip-expect-profile",
			Version: "1.0.0",
		},
		Plugins: []string{"file"},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "ctrl-1",
					Name: "Control One",
					ObservationDefinitions: []entities.ObservationDefinition{
						{
							Plugin: "file",
							Config: map[string]interface{}{},
							Expect: []string{"(invalid syntax"}, // invalid
						},
					},
				},
			},
		},
	}

	loader := &mockProfileLoaderForValidate{profile: profile}
	compiler := domainservices.NewProfileCompiler()

	uc := NewValidateProfileUseCase(loader, compiler)

	req := dto.ValidateProfileRequest{
		ProfilePath:          "skip.yaml",
		SkipExpectValidation: true, // Skip expect validation
	}

	resp, err := uc.Execute(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	// Should be valid because expect validation was skipped
	assert.True(t, resp.Valid)
	assert.Empty(t, resp.Errors)
}

func TestValidateProfileUseCase_Execute_MultiplePlugins(t *testing.T) {
	t.Parallel()

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "multi-plugin-profile",
			Version: "1.0.0",
		},
		Plugins: []string{"file", "http", "command"},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "ctrl-1",
					Name: "File Check",
					ObservationDefinitions: []entities.ObservationDefinition{
						{Plugin: "file", Config: map[string]interface{}{}},
					},
				},
				{
					ID:   "ctrl-2",
					Name: "HTTP Check",
					ObservationDefinitions: []entities.ObservationDefinition{
						{Plugin: "http", Config: map[string]interface{}{}},
						{Plugin: "http", Config: map[string]interface{}{}},
					},
				},
				{
					ID:   "ctrl-3",
					Name: "Command Check",
					ObservationDefinitions: []entities.ObservationDefinition{
						{Plugin: "command", Config: map[string]interface{}{}},
					},
				},
			},
		},
	}

	loader := &mockProfileLoaderForValidate{profile: profile}
	compiler := domainservices.NewProfileCompiler()

	uc := NewValidateProfileUseCase(loader, compiler)

	req := dto.ValidateProfileRequest{
		ProfilePath: "multi.yaml",
	}

	resp, err := uc.Execute(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Valid)
	assert.Equal(t, 3, resp.Stats.ControlCount)
	assert.Equal(t, 4, resp.Stats.ObservationCount)
	assert.Len(t, resp.Stats.PluginsUsed, 3)
}
