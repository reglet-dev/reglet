package entities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===== CONTROL ENTITY TESTS =====

func Test_Control_Validate(t *testing.T) {
	tests := []struct {
		name    string
		control Control
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid",
			control: Control{
				ID:   "ctrl-001",
				Name: "Test",
				ObservationDefinitions: []ObservationDefinition{
					{Plugin: "http"},
				},
			},
			wantErr: false,
		},
		{
			name: "missing_id",
			control: Control{
				Name: "Test",
				ObservationDefinitions: []ObservationDefinition{
					{Plugin: "http"},
				},
			},
			wantErr: true,
			errMsg:  "ID cannot be empty",
		},
		{
			name: "invalid_id",
			control: Control{
				ID:   "",
				Name: "Test",
				ObservationDefinitions: []ObservationDefinition{
					{Plugin: "http"},
				},
			},
			wantErr: true,
			errMsg:  "ID cannot be empty",
		},
		{
			name: "missing_name",
			control: Control{
				ID: "ctrl-001",
				ObservationDefinitions: []ObservationDefinition{
					{Plugin: "http"},
				},
			},
			wantErr: true,
			errMsg:  "name cannot be empty",
		},
		{
			name: "no_observations",
			control: Control{
				ID:           "ctrl-001",
				Name:         "Test",
				ObservationDefinitions: []ObservationDefinition{},
			},
			wantErr: true,
			errMsg:  "must have at least one observation",
		},
		{
			name: "invalid_severity",
			control: Control{
				ID:       "ctrl-001",
				Name:     "Test",
				Severity: "invalid",
				ObservationDefinitions: []ObservationDefinition{
					{Plugin: "http"},
				},
			},
			wantErr: true,
			errMsg:  "invalid severity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.control.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_Control_HasTag(t *testing.T) {
	ctrl := Control{
		ID:   "ctrl-001",
		Tags: []string{"production", "security"},
	}

	assert.True(t, ctrl.HasTag("production"))
	assert.True(t, ctrl.HasTag("security"))
	assert.False(t, ctrl.HasTag("development"))
}

func Test_Control_HasAnyTag(t *testing.T) {
	ctrl := Control{
		ID:   "ctrl-001",
		Tags: []string{"production", "security"},
	}

	assert.True(t, ctrl.HasAnyTag([]string{"production", "development"}))
	assert.True(t, ctrl.HasAnyTag([]string{"security"}))
	assert.False(t, ctrl.HasAnyTag([]string{"development", "staging"}))
	assert.False(t, ctrl.HasAnyTag([]string{}))
}

func Test_Control_MatchesSeverity(t *testing.T) {
	ctrl := Control{
		ID:       "ctrl-001",
		Severity: "high",
	}

	assert.True(t, ctrl.MatchesSeverity("high"))
	assert.False(t, ctrl.MatchesSeverity("low"))
}

func Test_Control_MatchesAnySeverity(t *testing.T) {
	ctrl := Control{
		ID:       "ctrl-001",
		Severity: "high",
	}

	assert.True(t, ctrl.MatchesAnySeverity([]string{"high", "critical"}))
	assert.False(t, ctrl.MatchesAnySeverity([]string{"low", "medium"}))
}

func Test_Control_HasDependency(t *testing.T) {
	ctrl := Control{
		ID:        "ctrl-001",
		DependsOn: []string{"ctrl-base", "ctrl-prereq"},
	}

	assert.True(t, ctrl.HasDependency("ctrl-base"))
	assert.True(t, ctrl.HasDependency("ctrl-prereq"))
	assert.False(t, ctrl.HasDependency("ctrl-other"))
}

func Test_Control_GetEffectiveTimeout(t *testing.T) {
	defaultTimeout := 5 * time.Second

	tests := []struct {
		name     string
		control  Control
		expected time.Duration
	}{
		{
			name:     "uses control timeout",
			control:  Control{Timeout: 10 * time.Second},
			expected: 10 * time.Second,
		},
		{
			name:     "falls back to default",
			control:  Control{Timeout: 0},
			expected: 5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.control.GetEffectiveTimeout(defaultTimeout)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_Control_ObservationCount(t *testing.T) {
	tests := []struct {
		name     string
		control  Control
		expected int
	}{
		{
			name:     "no observations",
			control:  Control{},
			expected: 0,
		},
		{
			name: "three observations",
			control: Control{
				ObservationDefinitions: []ObservationDefinition{
					{Plugin: "http"},
					{Plugin: "tcp"},
					{Plugin: "dns"},
				},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.control.ObservationCount())
		})
	}
}

func Test_Control_IsEmpty(t *testing.T) {
	assert.True(t, (&Control{}).IsEmpty())
	assert.False(t, (&Control{ID: "ctrl-001"}).IsEmpty())
	assert.False(t, (&Control{Name: "Test"}).IsEmpty())
}

// ===== PROFILE AGGREGATE TESTS =====

func Test_Profile_Validate(t *testing.T) {
	tests := []struct {
		name    string
		profile Profile
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid",
			profile: Profile{
				Metadata: ProfileMetadata{
					Name:    "Test",
					Version: "1.0.0",
				},
				Controls: ControlsSection{
					Items: []Control{
						{
							ID:   "ctrl-001",
							Name: "Test Control",
							ObservationDefinitions: []ObservationDefinition{
								{Plugin: "http"},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing_name",
			profile: Profile{
				Metadata: ProfileMetadata{
					Version: "1.0.0",
				},
			},
			wantErr: true,
			errMsg:  "name cannot be empty",
		},
		{
			name: "missing_version",
			profile: Profile{
				Metadata: ProfileMetadata{
					Name: "Test",
				},
			},
			wantErr: true,
			errMsg:  "version cannot be empty",
		},
		{
			name: "duplicate_control_ids",
			profile: Profile{
				Metadata: ProfileMetadata{
					Name:    "Test",
					Version: "1.0.0",
				},
				Controls: ControlsSection{
					Items: []Control{
						{
							ID:   "ctrl-001",
							Name: "First",
							ObservationDefinitions: []ObservationDefinition{
								{Plugin: "http"},
							},
						},
						{
							ID:   "ctrl-001",
							Name: "Duplicate",
							ObservationDefinitions: []ObservationDefinition{
								{Plugin: "tcp"},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "duplicate control ID",
		},
		{
			name: "invalid_dependency",
			profile: Profile{
				Metadata: ProfileMetadata{
					Name:    "Test",
					Version: "1.0.0",
				},
				Controls: ControlsSection{
					Items: []Control{
						{
							ID:        "ctrl-001",
							Name:      "Test",
							DependsOn: []string{"ctrl-nonexistent"},
							ObservationDefinitions: []ObservationDefinition{
								{Plugin: "http"},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "non-existent control",
		},
		{
			name: "circular_dependency",
			profile: Profile{
				Metadata: ProfileMetadata{
					Name:    "Test",
					Version: "1.0.0",
				},
				Controls: ControlsSection{
					Items: []Control{
						{
							ID:        "a",
							Name:      "A",
							DependsOn: []string{"b"},
							ObservationDefinitions: []ObservationDefinition{
								{Plugin: "http"},
							},
						},
						{
							ID:        "b",
							Name:      "B",
							DependsOn: []string{"a"},
							ObservationDefinitions: []ObservationDefinition{
								{Plugin: "http"},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "circular dependency detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.profile.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_Profile_AddControl(t *testing.T) {
	profile := Profile{
		Metadata: ProfileMetadata{
			Name:    "Test",
			Version: "1.0.0",
		},
		Controls: ControlsSection{
			Items: []Control{},
		},
	}

	// Add valid control
	ctrl1 := Control{
		ID:   "ctrl-001",
		Name: "Test",
		ObservationDefinitions: []ObservationDefinition{
			{Plugin: "http"},
		},
	}
	err := profile.AddControl(ctrl1)
	require.NoError(t, err)
	assert.Len(t, profile.Controls.Items, 1)

	// Add duplicate should fail
	err = profile.AddControl(ctrl1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	// Create a cycle via AddControl
	// Setup profile with A -> B
	profileWithCycle := Profile{
		Metadata: ProfileMetadata{Name: "CycleTest", Version: "1.0"},
		Controls: ControlsSection{
			Items: []Control{
				{
					ID:        "a",
					Name:      "A",
					DependsOn: []string{"b"},
					ObservationDefinitions: []ObservationDefinition{
						{Plugin: "http"},
					},
				},
			},
		},
	}

	// Try to add B -> A
	ctrlB := Control{
		ID:        "b",
		Name:      "B",
		DependsOn: []string{"a"},
		ObservationDefinitions: []ObservationDefinition{
			{Plugin: "http"},
		},
	}

	// Should fail due to cycle
	err = profileWithCycle.AddControl(ctrlB)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")

	// Ensure rollback happened (B should not be in Items)
	assert.Len(t, profileWithCycle.Controls.Items, 1)
	assert.Equal(t, "a", profileWithCycle.Controls.Items[0].ID)
}

func Test_Profile_GetControl(t *testing.T) {
	profile := Profile{
		Controls: ControlsSection{
			Items: []Control{
				{ID: "ctrl-001", Name: "First"},
				{ID: "ctrl-002", Name: "Second"},
			},
		},
	}

	ctrl := profile.GetControl("ctrl-001")
	require.NotNil(t, ctrl)
	assert.Equal(t, "First", ctrl.Name)

	ctrl = profile.GetControl("ctrl-nonexistent")
	assert.Nil(t, ctrl)
}

func Test_Profile_HasControl(t *testing.T) {
	profile := Profile{
		Controls: ControlsSection{
			Items: []Control{
				{ID: "ctrl-001"},
			},
		},
	}

	assert.True(t, profile.HasControl("ctrl-001"))
	assert.False(t, profile.HasControl("ctrl-002"))
}

func Test_Profile_ControlCount(t *testing.T) {
	profile := Profile{
		Controls: ControlsSection{
			Items: []Control{
				{ID: "ctrl-001"},
				{ID: "ctrl-002"},
				{ID: "ctrl-003"},
			},
		},
	}

	assert.Equal(t, 3, profile.ControlCount())
}

func Test_Profile_SelectControlsByTags(t *testing.T) {
	profile := Profile{
		Controls: ControlsSection{
			Items: []Control{
				{ID: "ctrl-001", Tags: []string{"production", "security"}},
				{ID: "ctrl-002", Tags: []string{"development"}},
				{ID: "ctrl-003", Tags: []string{"production", "compliance"}},
			},
		},
	}

	selected := profile.SelectControlsByTags([]string{"production"})
	assert.Len(t, selected, 2)

	selected = profile.SelectControlsByTags([]string{})
	assert.Len(t, selected, 3)
}

func Test_Profile_SelectControlsBySeverity(t *testing.T) {
	profile := Profile{
		Controls: ControlsSection{
			Items: []Control{
				{ID: "ctrl-001", Severity: "high"},
				{ID: "ctrl-002", Severity: "low"},
				{ID: "ctrl-003", Severity: "critical"},
			},
		},
	}

	selected := profile.SelectControlsBySeverity([]string{"high", "critical"})
	assert.Len(t, selected, 2)

	selected = profile.SelectControlsBySeverity([]string{})
	assert.Len(t, selected, 3)
}

func Test_Profile_ExcludeControlsByID(t *testing.T) {
	profile := Profile{
		Controls: ControlsSection{
			Items: []Control{
				{ID: "ctrl-001"},
				{ID: "ctrl-002"},
				{ID: "ctrl-003"},
			},
		},
	}

	selected := profile.ExcludeControlsByID([]string{"ctrl-002"})
	assert.Len(t, selected, 2)
	assert.Equal(t, "ctrl-001", selected[0].ID)
	assert.Equal(t, "ctrl-003", selected[1].ID)
}

func Test_Profile_ApplyDefaults(t *testing.T) {
	profile := Profile{
		Controls: ControlsSection{
			Defaults: &ControlDefaults{
				Severity: "medium",
				Owner:    "platform",
				Tags:     []string{"default-tag"},
				Timeout:  10 * time.Second,
			},
			Items: []Control{
				{
					ID:       "ctrl-001",
					Name:     "Has overrides",
					Severity: "high",
					Owner:    "security",
					Tags:     []string{"custom-tag"},
					Timeout:  5 * time.Second,
				},
				{
					ID:   "ctrl-002",
					Name: "Uses defaults",
				},
			},
		},
	}

	profile.ApplyDefaults()

	// First control keeps its overrides
	ctrl1 := profile.Controls.Items[0]
	assert.Equal(t, "high", ctrl1.Severity)
	assert.Equal(t, "security", ctrl1.Owner)
	assert.Equal(t, 5*time.Second, ctrl1.Timeout)
	// Tags should be merged
	assert.Len(t, ctrl1.Tags, 2) // custom-tag + default-tag

	// Second control gets defaults
	ctrl2 := profile.Controls.Items[1]
	assert.Equal(t, "medium", ctrl2.Severity)
	assert.Equal(t, "platform", ctrl2.Owner)
	assert.Equal(t, 10*time.Second, ctrl2.Timeout)
	assert.Contains(t, ctrl2.Tags, "default-tag")
}
