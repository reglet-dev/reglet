package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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

func TestProfile_Validate(t *testing.T) {
	// Valid profile
	p := Profile{
		Metadata: ProfileMetadata{Name: "test"},
		Controls: ControlsSection{Items: []Control{{ID: "c1", Name: "C1", Observations: []Observation{{}}}}},
	}
	assert.NoError(t, p.Validate())

	// Empty name
	pInvalid := p
	pInvalid.Metadata.Name = ""
	assert.Error(t, pInvalid.Validate())

	// No controls
	pEmpty := Profile{Metadata: ProfileMetadata{Name: "test"}}
	assert.Error(t, pEmpty.Validate())
}

func TestProfile_GetControl(t *testing.T) {
	p := Profile{
		Controls: ControlsSection{Items: []Control{
			{ID: "c1"},
			{ID: "c2"},
		}},
	}

	assert.NotNil(t, p.GetControl("c1"))
	assert.Nil(t, p.GetControl("c3"))
}

func TestProfile_HasControl(t *testing.T) {
	p := Profile{
		Controls: ControlsSection{Items: []Control{{ID: "c1"}}},
	}
	assert.True(t, p.HasControl("c1"))
	assert.False(t, p.HasControl("c2"))
}

func TestProfile_ControlCount(t *testing.T) {
	p := Profile{
		Controls: ControlsSection{Items: []Control{{ID: "c1"}, {ID: "c2"}}},
	}
	assert.Equal(t, 2, p.ControlCount())
}

func TestProfile_SelectControlsByTags(t *testing.T) {
	p := Profile{
		Controls: ControlsSection{Items: []Control{
			{ID: "c1", Tags: []string{"prod"}},
			{ID: "c2", Tags: []string{"dev"}},
			{ID: "c3", Tags: []string{"prod", "security"}},
		}},
	}

	selected := p.SelectControlsByTags([]string{"prod"})
	assert.Len(t, selected, 2)
	assert.Equal(t, "c1", selected[0].ID)
	assert.Equal(t, "c3", selected[1].ID)
}

func TestProfile_SelectControlsBySeverity(t *testing.T) {
	p := Profile{
		Controls: ControlsSection{Items: []Control{
			{ID: "c1", Severity: "high"},
			{ID: "c2", Severity: "low"},
			{ID: "c3", Severity: "critical"},
		}},
	}

	selected := p.SelectControlsBySeverity([]string{"high", "critical"})
	assert.Len(t, selected, 2)
	assert.Equal(t, "c1", selected[0].ID)
	assert.Equal(t, "c3", selected[1].ID)
}

func TestProfile_ExcludeControlsByID(t *testing.T) {
	p := Profile{
		Controls: ControlsSection{Items: []Control{
			{ID: "c1"},
			{ID: "c2"},
			{ID: "c3"},
		}},
	}

	selected := p.ExcludeControlsByID([]string{"c2"})
	assert.Len(t, selected, 2)
	assert.Equal(t, "c1", selected[0].ID)
	assert.Equal(t, "c3", selected[1].ID)
}

func TestProfile_ApplyDefaults(t *testing.T) {
	p := Profile{
		Controls: ControlsSection{
			Defaults: &ControlDefaults{
				Severity: "medium",
				Owner:    "platform",
				Timeout:  30 * time.Second,
				Tags:     []string{"default"},
			},
			Items: []Control{
				{ID: "c1"},                                     // Should inherit all
				{ID: "c2", Severity: "high"},                   // Should override severity
				{ID: "c3", Tags: []string{"custom"}},           // Should merge tags? No, current logic is "if empty"
				{ID: "c4", Timeout: 10 * time.Second},          // Should override timeout
			},
		},
	}

	p.ApplyDefaults()

	// c1
	c1 := p.GetControl("c1")
	assert.Equal(t, "medium", c1.Severity)
	assert.Equal(t, "platform", c1.Owner)
	assert.Equal(t, 30*time.Second, c1.Timeout)
	assert.Contains(t, c1.Tags, "default")

	// c2
	c2 := p.GetControl("c2")
	assert.Equal(t, "high", c2.Severity)
	assert.Equal(t, "platform", c2.Owner)

	// c3 - current logic: defaults are merged with control tags
	// so c3 tags should contain both "custom" and "default"
	c3 := p.GetControl("c3")
	assert.Contains(t, c3.Tags, "custom")
	assert.Contains(t, c3.Tags, "default")

	// c4
	c4 := p.GetControl("c4")
	assert.Equal(t, 10*time.Second, c4.Timeout)
}