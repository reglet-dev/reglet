package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/whiskeyjimbo/reglet/internal/domain/valueobjects"
)

func TestControl_Validate(t *testing.T) {
	tests := []struct {
		name    string
		control Control
		wantErr bool
	}{
		{
			name: "valid",
			control: Control{
				ID:           "valid-id",
				Name:         "Valid Control",
				Observations: []Observation{{Plugin: "test", Config: map[string]interface{}{}}},
			},
			wantErr: false,
		},
		{
			name: "missing id",
			control: Control{
				Name:         "Valid Control",
				Observations: []Observation{{Plugin: "test"}},
			},
			wantErr: true,
		},
		{
			name: "invalid id",
			control: Control{
				ID:           "   ",
				Name:         "Valid Control",
				Observations: []Observation{{Plugin: "test"}},
			},
			wantErr: true,
		},
		{
			name: "missing name",
			control: Control{
				ID:           "valid-id",
				Observations: []Observation{{Plugin: "test"}},
			},
			wantErr: true,
		},
		{
			name: "no observations",
			control: Control{
				ID:   "valid-id",
				Name: "Valid Control",
			},
			wantErr: true,
		},
		{
			name: "invalid severity",
			control: Control{
				ID:           "valid-id",
				Name:         "Valid Control",
				Severity:     "super-critical",
				Observations: []Observation{{Plugin: "test"}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.control.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestControl_HasTag(t *testing.T) {
	ctrl := Control{Tags: []string{"prod", "security"}}
	assert.True(t, ctrl.HasTag("prod"))
	assert.True(t, ctrl.HasTag("security"))
	assert.False(t, ctrl.HasTag("dev"))
}

func TestControl_MatchesSeverity(t *testing.T) {
	ctrlHigh := Control{Severity: "high"}
	ctrlLow := Control{Severity: "low"}

	sevHigh := valueobjects.MustNewSeverity("high")
	sevMedium := valueobjects.MustNewSeverity("medium")

	assert.True(t, ctrlHigh.MatchesSeverity(sevMedium))
	assert.True(t, ctrlHigh.MatchesSeverity(sevHigh))
	assert.False(t, ctrlLow.MatchesSeverity(sevMedium))
}

func TestControl_HasAnyTag(t *testing.T) {
	ctrl := Control{Tags: []string{"prod", "security"}}
	assert.True(t, ctrl.HasAnyTag([]string{"dev", "prod"}), "should match prod")
	assert.False(t, ctrl.HasAnyTag([]string{"dev", "qa"}), "should not match")
	assert.False(t, ctrl.HasAnyTag([]string{}), "empty tags should not match")
}

func TestControl_MatchesAnySeverity(t *testing.T) {
	ctrl := Control{Severity: "high"}
	assert.True(t, ctrl.MatchesAnySeverity([]string{"critical", "high"}), "should match high")
	assert.False(t, ctrl.MatchesAnySeverity([]string{"low", "medium"}), "should not match")
	assert.False(t, ctrl.MatchesAnySeverity([]string{}), "empty severities should not match")
}

func TestControl_HasDependency(t *testing.T) {
	ctrl := Control{DependsOn: []string{"dep1", "dep2"}}
	assert.True(t, ctrl.HasDependency("dep1"))
	assert.False(t, ctrl.HasDependency("dep3"))
}

func TestControl_GetEffectiveTimeout(t *testing.T) {
	ctrlWithTimeout := Control{Timeout: 5 * time.Second}
	ctrlNoTimeout := Control{}

	defaultTimeout := 10 * time.Second

	assert.Equal(t, 5*time.Second, ctrlWithTimeout.GetEffectiveTimeout(defaultTimeout))
	assert.Equal(t, 10*time.Second, ctrlNoTimeout.GetEffectiveTimeout(defaultTimeout))
}

func TestControl_ObservationCount(t *testing.T) {
	ctrl := Control{Observations: []Observation{{}, {}}}
	assert.Equal(t, 2, ctrl.ObservationCount())
}

func TestControl_IsEmpty(t *testing.T) {
	empty := Control{}
	assert.True(t, empty.IsEmpty())

	notEmpty := Control{ID: "c1"}
	assert.False(t, notEmpty.IsEmpty())
}
