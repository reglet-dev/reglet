package config

import (
	"testing"

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

func TestProfile_AddControl(t *testing.T) {
	profile := Profile{}
	ctrl1 := Control{ID: "c1", Name: "C1", Observations: []Observation{{}}}

	err := profile.AddControl(ctrl1)
	assert.NoError(t, err)
	assert.Len(t, profile.Controls.Items, 1)

	// Duplicate
	err = profile.AddControl(ctrl1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")

	// Invalid
	invalidCtrl := Control{ID: "", Name: "Invalid"}
	err = profile.AddControl(invalidCtrl)
	assert.Error(t, err)
}
