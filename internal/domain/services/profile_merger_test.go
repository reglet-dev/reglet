package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/reglet-dev/reglet/internal/domain/entities"
)

func Test_ProfileMerger_MergeMetadata_OverlayWins(t *testing.T) {
	t.Parallel()
	merger := NewProfileMerger()

	base := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:        "base-name",
			Version:     "1.0.0",
			Description: "Base description",
		},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{ID: "ctrl-1", Name: "Control 1", ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "file"}}},
			},
		},
	}

	overlay := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "overlay-name",
			Version: "2.0.0",
			// Description is empty - should inherit from base
		},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{ID: "ctrl-2", Name: "Control 2", ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "http"}}},
			},
		},
	}

	result := merger.Merge(base, overlay)

	assert.Equal(t, "overlay-name", result.Metadata.Name)
	assert.Equal(t, "2.0.0", result.Metadata.Version)
	assert.Equal(t, "Base description", result.Metadata.Description, "Empty overlay field should inherit from base")
}

func Test_ProfileMerger_MergeVars_DeepMerge(t *testing.T) {
	t.Parallel()
	merger := NewProfileMerger()

	base := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "base", Version: "1.0.0"},
		Vars: map[string]interface{}{
			"base_only":  "value1",
			"shared_key": "base_value",
			"timeout":    30,
		},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{ID: "ctrl-1", Name: "Control 1", ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "file"}}},
			},
		},
	}

	overlay := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "overlay", Version: "2.0.0"},
		Vars: map[string]interface{}{
			"overlay_only": "value2",
			"shared_key":   "overlay_value", // Should override base
		},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{ID: "ctrl-2", Name: "Control 2", ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "http"}}},
			},
		},
	}

	result := merger.Merge(base, overlay)

	require.NotNil(t, result.Vars)
	assert.Equal(t, "value1", result.Vars["base_only"])
	assert.Equal(t, "value2", result.Vars["overlay_only"])
	assert.Equal(t, "overlay_value", result.Vars["shared_key"], "Overlay should win on conflict")
	assert.Equal(t, 30, result.Vars["timeout"])
}

func Test_ProfileMerger_MergePlugins_Deduplicate(t *testing.T) {
	t.Parallel()
	merger := NewProfileMerger()

	base := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "base", Version: "1.0.0"},
		Plugins:  []string{"reglet/file@1.0", "reglet/http@1.0"},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{ID: "ctrl-1", Name: "Control 1", ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "file"}}},
			},
		},
	}

	overlay := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "overlay", Version: "2.0.0"},
		Plugins:  []string{"reglet/http@1.0", "reglet/dns@1.0"}, // http is duplicate
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{ID: "ctrl-2", Name: "Control 2", ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "dns"}}},
			},
		},
	}

	result := merger.Merge(base, overlay)

	// Should preserve order: base first, then new overlay plugins
	expected := []string{"reglet/file@1.0", "reglet/http@1.0", "reglet/dns@1.0"}
	assert.Equal(t, expected, result.Plugins)
}

func Test_ProfileMerger_MergeDefaults_TagsConcatenate(t *testing.T) {
	t.Parallel()
	merger := NewProfileMerger()

	base := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "base", Version: "1.0.0"},
		Controls: entities.ControlsSection{
			Defaults: &entities.ControlDefaults{
				Severity: "high",
				Owner:    "base-owner",
				Tags:     []string{"security", "compliance"},
				Timeout:  30 * time.Second,
			},
			Items: []entities.Control{
				{ID: "ctrl-1", Name: "Control 1", ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "file"}}},
			},
		},
	}

	overlay := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "overlay", Version: "2.0.0"},
		Controls: entities.ControlsSection{
			Defaults: &entities.ControlDefaults{
				Severity: "critical", // Override
				// Owner is empty - inherit from base
				Tags:    []string{"production", "security"}, // security is duplicate
				Timeout: 60 * time.Second,                   // Override
			},
			Items: []entities.Control{
				{ID: "ctrl-2", Name: "Control 2", ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "http"}}},
			},
		},
	}

	result := merger.Merge(base, overlay)

	require.NotNil(t, result.Controls.Defaults)
	assert.Equal(t, "critical", result.Controls.Defaults.Severity)
	assert.Equal(t, "base-owner", result.Controls.Defaults.Owner, "Empty overlay field inherits from base")
	assert.Equal(t, 60*time.Second, result.Controls.Defaults.Timeout)

	// Tags should be deduplicated and concatenated
	assert.Contains(t, result.Controls.Defaults.Tags, "security")
	assert.Contains(t, result.Controls.Defaults.Tags, "compliance")
	assert.Contains(t, result.Controls.Defaults.Tags, "production")
	assert.Len(t, result.Controls.Defaults.Tags, 3, "Should have 3 unique tags")
}

func Test_ProfileMerger_MergeControlItems_SameIDReplaces(t *testing.T) {
	t.Parallel()
	merger := NewProfileMerger()

	base := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "base", Version: "1.0.0"},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:       "ssh-config",
					Name:     "SSH Configuration",
					Severity: "high",
					ObservationDefinitions: []entities.ObservationDefinition{
						{Plugin: "file", Config: map[string]interface{}{"path": "/etc/ssh/sshd_config"}},
					},
				},
			},
		},
	}

	overlay := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "overlay", Version: "2.0.0"},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:       "ssh-config", // Same ID - should replace entirely
					Name:     "SSH Configuration (Production)",
					Severity: "critical",
					ObservationDefinitions: []entities.ObservationDefinition{
						{Plugin: "file", Config: map[string]interface{}{"path": "/etc/ssh/sshd_config", "mode": "strict"}},
					},
				},
			},
		},
	}

	result := merger.Merge(base, overlay)

	require.Len(t, result.Controls.Items, 1)
	ctrl := result.Controls.Items[0]
	assert.Equal(t, "ssh-config", ctrl.ID)
	assert.Equal(t, "SSH Configuration (Production)", ctrl.Name, "Name should be from overlay")
	assert.Equal(t, "critical", ctrl.Severity, "Severity should be from overlay")
	assert.Contains(t, ctrl.ObservationDefinitions[0].Config, "mode", "Config should be from overlay")
}

func Test_ProfileMerger_MergeControlItems_NewIDAppends(t *testing.T) {
	t.Parallel()
	merger := NewProfileMerger()

	base := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "base", Version: "1.0.0"},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{ID: "ctrl-1", Name: "Control 1", ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "file"}}},
				{ID: "ctrl-2", Name: "Control 2", ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "http"}}},
			},
		},
	}

	overlay := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "overlay", Version: "2.0.0"},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{ID: "ctrl-3", Name: "Control 3", ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "dns"}}}, // New
			},
		},
	}

	result := merger.Merge(base, overlay)

	require.Len(t, result.Controls.Items, 3)
	// Order: base controls first, then new overlay controls
	assert.Equal(t, "ctrl-1", result.Controls.Items[0].ID)
	assert.Equal(t, "ctrl-2", result.Controls.Items[1].ID)
	assert.Equal(t, "ctrl-3", result.Controls.Items[2].ID)
}

func Test_ProfileMerger_MergeAll_LeftToRightPrecedence(t *testing.T) {
	t.Parallel()
	merger := NewProfileMerger()

	parent1 := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "parent1", Version: "1.0.0"},
		Vars:     map[string]interface{}{"key": "parent1_value"},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{ID: "ctrl-1", Name: "From Parent1", ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "file"}}},
			},
		},
	}

	parent2 := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "parent2", Version: "2.0.0"},
		Vars:     map[string]interface{}{"key": "parent2_value"}, // Should override parent1
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{ID: "ctrl-1", Name: "From Parent2", ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "http"}}}, // Override
				{ID: "ctrl-2", Name: "Control 2", ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "dns"}}},
			},
		},
	}

	current := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "current", Version: "3.0.0"},
		Vars:     map[string]interface{}{"key": "current_value"}, // Should override all
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{ID: "ctrl-3", Name: "Control 3", ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "smtp"}}},
			},
		},
	}

	result := merger.MergeAll([]*entities.Profile{parent1, parent2}, current)

	// Metadata: current wins
	assert.Equal(t, "current", result.Metadata.Name)
	assert.Equal(t, "3.0.0", result.Metadata.Version)

	// Vars: current wins
	assert.Equal(t, "current_value", result.Vars["key"])

	// Controls: merged in order
	require.Len(t, result.Controls.Items, 3)
	assert.Equal(t, "From Parent2", result.Controls.Items[0].Name, "ctrl-1 should be from parent2")
	assert.Equal(t, "ctrl-2", result.Controls.Items[1].ID)
	assert.Equal(t, "ctrl-3", result.Controls.Items[2].ID)
}

func Test_ProfileMerger_ExtendsNotPropagated(t *testing.T) {
	t.Parallel()
	merger := NewProfileMerger()

	base := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "base", Version: "1.0.0"},
		Extends:  []string{"./grandparent.yaml"},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{ID: "ctrl-1", Name: "Control 1", ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "file"}}},
			},
		},
	}

	overlay := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "overlay", Version: "2.0.0"},
		Extends:  []string{"./another-parent.yaml"},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{ID: "ctrl-2", Name: "Control 2", ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "http"}}},
			},
		},
	}

	result := merger.Merge(base, overlay)

	assert.Nil(t, result.Extends, "Extends should NOT be propagated after merge")
}

func Test_ProfileMerger_Immutability_InputsNotModified(t *testing.T) {
	t.Parallel()
	merger := NewProfileMerger()

	base := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "base", Version: "1.0.0"},
		Vars:     map[string]interface{}{"key": "original"},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{ID: "ctrl-1", Name: "Original Name", ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "file"}}},
			},
		},
	}

	overlay := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "overlay", Version: "2.0.0"},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{ID: "ctrl-1", Name: "Modified Name", ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "http"}}},
			},
		},
	}

	// Store original values
	originalBaseName := base.Controls.Items[0].Name
	originalBaseVarValue := base.Vars["key"]

	// Perform merge
	result := merger.Merge(base, overlay)

	// Verify inputs are not modified
	assert.Equal(t, originalBaseName, base.Controls.Items[0].Name, "Base should not be modified")
	assert.Equal(t, originalBaseVarValue, base.Vars["key"], "Base vars should not be modified")

	// Verify result is different
	assert.Equal(t, "Modified Name", result.Controls.Items[0].Name)
}

func Test_ProfileMerger_EmptyParents_ReturnsCurrentCopy(t *testing.T) {
	t.Parallel()
	merger := NewProfileMerger()

	current := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "current", Version: "1.0.0"},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{ID: "ctrl-1", Name: "Control 1", ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "file"}}},
			},
		},
	}

	result := merger.MergeAll([]*entities.Profile{}, current)

	assert.Equal(t, current.Metadata.Name, result.Metadata.Name)
	assert.Len(t, result.Controls.Items, 1)

	// Verify it's a copy, not the same pointer
	result.Metadata.Name = "modified"
	assert.Equal(t, "current", current.Metadata.Name, "Should not modify original")
}

func Test_ProfileMerger_NilDefaults_Handled(t *testing.T) {
	t.Parallel()
	merger := NewProfileMerger()

	base := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "base", Version: "1.0.0"},
		Controls: entities.ControlsSection{
			Defaults: nil, // No defaults
			Items: []entities.Control{
				{ID: "ctrl-1", Name: "Control 1", ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "file"}}},
			},
		},
	}

	overlay := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "overlay", Version: "2.0.0"},
		Controls: entities.ControlsSection{
			Defaults: &entities.ControlDefaults{
				Severity: "high",
			},
			Items: []entities.Control{
				{ID: "ctrl-2", Name: "Control 2", ObservationDefinitions: []entities.ObservationDefinition{{Plugin: "http"}}},
			},
		},
	}

	result := merger.Merge(base, overlay)

	require.NotNil(t, result.Controls.Defaults)
	assert.Equal(t, "high", result.Controls.Defaults.Severity)
}
