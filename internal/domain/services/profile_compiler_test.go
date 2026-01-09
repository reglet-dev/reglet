package services

import (
	"testing"
	"time"

	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ProfileCompiler_Compile_Success(t *testing.T) {
	compiler := NewProfileCompiler()

	raw := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "test-profile",
			Version: "1.0.0",
		},
		Plugins: []string{"file", "http"},
		Vars: map[string]interface{}{
			"env": "prod",
		},
		Controls: entities.ControlsSection{
			Defaults: &entities.ControlDefaults{
				Severity: "medium",
				Owner:    "security-team",
				Tags:     []string{"compliance"},
				Timeout:  30 * time.Second,
			},
			Items: []entities.Control{
				{
					ID:   "C-001",
					Name: "Check SSL",
					ObservationDefinitions: []entities.ObservationDefinition{
						{Plugin: "http", Config: map[string]interface{}{"url": "https://example.com"}},
					},
				},
				{
					ID:       "C-002",
					Name:     "Check Config",
					Severity: "high", // Override default
					Tags:     []string{"config"},
					ObservationDefinitions: []entities.ObservationDefinition{
						{Plugin: "file", Config: map[string]interface{}{"path": "/etc/app.conf"}},
					},
				},
			},
		},
	}

	validated, err := compiler.Compile(raw)
	require.NoError(t, err)
	require.NotNil(t, validated)

	// Verify it's a ValidatedProfile
	assert.True(t, validated.IsValidated())

	// Verify defaults were applied to C-001
	ctrl1 := validated.GetControl("C-001")
	require.NotNil(t, ctrl1)
	assert.Equal(t, "medium", ctrl1.Severity, "Should inherit default severity")
	assert.Equal(t, "security-team", ctrl1.Owner, "Should inherit default owner")
	assert.Contains(t, ctrl1.Tags, "compliance", "Should inherit default tags")
	assert.Equal(t, 30*time.Second, ctrl1.Timeout, "Should inherit default timeout")

	// Verify C-002 kept its overrides but merged tags
	ctrl2 := validated.GetControl("C-002")
	require.NotNil(t, ctrl2)
	assert.Equal(t, "high", ctrl2.Severity, "Should keep explicit severity")
	assert.Contains(t, ctrl2.Tags, "compliance", "Should have default tag")
	assert.Contains(t, ctrl2.Tags, "config", "Should have explicit tag")

	// Verify original profile was NOT mutated
	origCtrl1 := raw.GetControl("C-001")
	assert.Empty(t, origCtrl1.Severity, "Original should not have defaults applied")
	assert.Empty(t, origCtrl1.Owner, "Original should not have defaults applied")
	assert.Empty(t, origCtrl1.Tags, "Original should not have defaults applied")
}

func Test_ProfileCompiler_Compile_NilProfile(t *testing.T) {
	compiler := NewProfileCompiler()

	validated, err := compiler.Compile(nil)
	assert.Error(t, err)
	assert.Nil(t, validated)
	assert.Contains(t, err.Error(), "cannot compile nil profile")
}

func Test_ProfileCompiler_Compile_ValidationFails(t *testing.T) {
	compiler := NewProfileCompiler()

	tests := []struct {
		name    string
		profile *entities.Profile
		errMsg  string
	}{
		{
			name: "missing profile name",
			profile: &entities.Profile{
				Metadata: entities.ProfileMetadata{
					Version: "1.0.0",
				},
				Controls: entities.ControlsSection{
					Items: []entities.Control{
						{ID: "C-001", Name: "Test", ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "file"}}},
					},
				},
			},
			errMsg: "profile name cannot be empty",
		},
		{
			name: "missing profile version",
			profile: &entities.Profile{
				Metadata: entities.ProfileMetadata{
					Name: "test",
				},
				Controls: entities.ControlsSection{
					Items: []entities.Control{
						{ID: "C-001", Name: "Test", ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "file"}}},
					},
				},
			},
			errMsg: "profile version cannot be empty",
		},
		{
			name: "no controls",
			profile: &entities.Profile{
				Metadata: entities.ProfileMetadata{
					Name:    "test",
					Version: "1.0.0",
				},
				Controls: entities.ControlsSection{
					Items: []entities.Control{},
				},
			},
			errMsg: "at least one control is required",
		},
		{
			name: "duplicate control IDs",
			profile: &entities.Profile{
				Metadata: entities.ProfileMetadata{
					Name:    "test",
					Version: "1.0.0",
				},
				Controls: entities.ControlsSection{
					Items: []entities.Control{
						{ID: "C-001", Name: "Test1", ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "file"}}},
						{ID: "C-001", Name: "Test2", ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "http"}}},
					},
				},
			},
			errMsg: "duplicate control ID",
		},
		{
			name: "non-existent dependency",
			profile: &entities.Profile{
				Metadata: entities.ProfileMetadata{
					Name:    "test",
					Version: "1.0.0",
				},
				Controls: entities.ControlsSection{
					Items: []entities.Control{
						{ID: "C-001", Name: "Test", DependsOn: []string{"C-999"}, ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "file"}}},
					},
				},
			},
			errMsg: "depends on non-existent control",
		},
		{
			name: "circular dependency",
			profile: &entities.Profile{
				Metadata: entities.ProfileMetadata{
					Name:    "test",
					Version: "1.0.0",
				},
				Controls: entities.ControlsSection{
					Items: []entities.Control{
						{ID: "C-001", Name: "Test1", DependsOn: []string{"C-002"}, ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "file"}}},
						{ID: "C-002", Name: "Test2", DependsOn: []string{"C-001"}, ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "http"}}},
					},
				},
			},
			errMsg: "circular dependency",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validated, err := compiler.Compile(tt.profile)
			assert.Error(t, err)
			assert.Nil(t, validated)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func Test_ProfileCompiler_DeepCopyPreventsMutation(t *testing.T) {
	compiler := NewProfileCompiler()

	raw := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "test-profile",
			Version: "1.0.0",
		},
		Plugins: []string{"file"},
		Vars: map[string]interface{}{
			"key": "original-value",
		},
		Controls: entities.ControlsSection{
			Defaults: &entities.ControlDefaults{
				Severity: "low",
				Tags:     []string{"original-tag"},
			},
			Items: []entities.Control{
				{
					ID:        "C-001",
					Name:      "Original Name",
					Tags:      []string{"original"},
					DependsOn: []string{},
					ObservationDefinitions: []entities.ObservationDefinition{
						{Plugin: "file", Config: map[string]interface{}{"path": "/original"}},
					},
				},
			},
		},
	}

	// Store original values
	origPlugins := len(raw.Plugins)
	origVarValue := raw.Vars["key"]
	origControlName := raw.Controls.Items[0].Name
	origTag := raw.Controls.Items[0].Tags[0]
	origDefaultSeverity := raw.Controls.Defaults.Severity

	// Compile
	validated, err := compiler.Compile(raw)
	require.NoError(t, err)
	require.NotNil(t, validated)

	// Modify the validated profile's underlying data (if we could - this tests deep copy)
	// The compiler should have created a copy, so modifying validated shouldn't affect raw
	validatedCtrl := validated.GetControl("C-001")
	require.NotNil(t, validatedCtrl)

	// Verify original is unchanged
	assert.Len(t, raw.Plugins, origPlugins, "Original plugins should be unchanged")
	assert.Equal(t, origVarValue, raw.Vars["key"], "Original vars should be unchanged")
	assert.Equal(t, origControlName, raw.Controls.Items[0].Name, "Original control name should be unchanged")
	assert.Equal(t, origTag, raw.Controls.Items[0].Tags[0], "Original tags should be unchanged")
	assert.Equal(t, origDefaultSeverity, raw.Controls.Defaults.Severity, "Original defaults should be unchanged")

	// The compiled version should have defaults applied (different from original)
	assert.NotEmpty(t, validatedCtrl.Severity, "Compiled control should have defaults applied")
	assert.Empty(t, raw.Controls.Items[0].Severity, "Original control should NOT have defaults applied")
}

func Test_ProfileCompiler_ApplyDefaults_NoDefaults(t *testing.T) {
	compiler := NewProfileCompiler()

	raw := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "test-profile",
			Version: "1.0.0",
		},
		Controls: entities.ControlsSection{
			Defaults: nil, // No defaults
			Items: []entities.Control{
				{
					ID:   "C-001",
					Name: "Test",
					ObservationDefinitions: []entities.ObservationDefinition{
						{Plugin: "file"},
					},
				},
			},
		},
	}

	validated, err := compiler.Compile(raw)
	require.NoError(t, err)
	require.NotNil(t, validated)

	// Control should remain unchanged
	ctrl := validated.GetControl("C-001")
	require.NotNil(t, ctrl)
	assert.Empty(t, ctrl.Severity)
	assert.Empty(t, ctrl.Owner)
	assert.Empty(t, ctrl.Tags)
	assert.Zero(t, ctrl.Timeout)
}

func Test_ProfileCompiler_ImplementsProfileReader(t *testing.T) {
	compiler := NewProfileCompiler()

	raw := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "test-profile",
			Version: "1.0.0",
		},
		Plugins: []string{"file", "http"},
		Vars: map[string]interface{}{
			"env": "test",
		},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "C-001",
					Name: "Test Control",
					Tags: []string{"tag1", "tag2"},
					ObservationDefinitions: []entities.ObservationDefinition{
						{Plugin: "file"},
					},
				},
				{
					ID:   "C-002",
					Name: "Another Control",
					ObservationDefinitions: []entities.ObservationDefinition{
						{Plugin: "http"},
					},
				},
			},
		},
	}

	validated, err := compiler.Compile(raw)
	require.NoError(t, err)

	// Verify it implements ProfileReader interface
	var reader entities.ProfileReader = validated

	// Test interface methods
	assert.Equal(t, "test-profile", reader.GetMetadata().Name)
	assert.Equal(t, []string{"file", "http"}, reader.GetPlugins())
	assert.Equal(t, "test", reader.GetVars()["env"])
	assert.Equal(t, 2, reader.ControlCount())
	assert.NotNil(t, reader.GetControl("C-001"))
	assert.True(t, reader.HasControl("C-001"))
	assert.False(t, reader.HasControl("C-999"))
	assert.Len(t, reader.GetAllControls(), 2)

	// Test filtering methods
	selected := reader.SelectControlsByTags([]string{"tag1"})
	assert.Len(t, selected, 1)
	assert.Equal(t, "C-001", selected[0].ID)
}
