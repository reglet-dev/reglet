package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
)

func TestValidate_Valid(t *testing.T) {
	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "test-profile",
			Version: "1.0.0",
		},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "test-control",
					Name: "Test Control",
					ObservationDefinitions: []entities.ObservationDefinition{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/etc/test",
							},
						},
					},
				},
			},
		},
	}

	validator := NewProfileValidator()
	err := validator.Validate(profile)
	assert.NoError(t, err)
}

func TestValidate_MissingName(t *testing.T) {
	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Version: "1.0.0",
		},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "test-control",
					Name: "Test Control",
					ObservationDefinitions: []entities.ObservationDefinition{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/etc/test",
							},
						},
					},
				},
			},
		},
	}

	validator := NewProfileValidator()
	err := validator.Validate(profile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "profile name is required")
}

func TestValidate_InvalidVersion(t *testing.T) {
	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "test-profile",
			Version: "invalid",
		},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "test-control",
					Name: "Test Control",
					ObservationDefinitions: []entities.ObservationDefinition{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/etc/test",
							},
						},
					},
				},
			},
		},
	}

	validator := NewProfileValidator()
	err := validator.Validate(profile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "version")
	assert.Contains(t, err.Error(), "not valid")
}

func TestValidate_MissingControlID(t *testing.T) {
	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "test-profile",
			Version: "1.0.0",
		},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					Name: "Test Control",
					ObservationDefinitions: []entities.ObservationDefinition{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/etc/test",
							},
						},
					},
				},
			},
		},
	}

	validator := NewProfileValidator()
	err := validator.Validate(profile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "control ID is required")
}

func TestValidate_InvalidControlID(t *testing.T) {
	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "test-profile",
			Version: "1.0.0",
		},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "invalid id with spaces",
					Name: "Test Control",
					ObservationDefinitions: []entities.ObservationDefinition{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/etc/test",
							},
						},
					},
				},
			},
		},
	}

	validator := NewProfileValidator()
	err := validator.Validate(profile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "control ID")
	assert.Contains(t, err.Error(), "invalid")
}

func TestValidate_DuplicateControlIDs(t *testing.T) {
	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "test-profile",
			Version: "1.0.0",
		},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "test-control",
					Name: "Test Control 1",
					ObservationDefinitions: []entities.ObservationDefinition{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/etc/test1",
							},
						},
					},
				},
				{
					ID:   "test-control",
					Name: "Test Control 2",
					ObservationDefinitions: []entities.ObservationDefinition{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/etc/test2",
							},
						},
					},
				},
			},
		},
	}

	validator := NewProfileValidator()
	err := validator.Validate(profile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate control ID")
}

func TestValidate_NoObservations(t *testing.T) {
	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "test-profile",
			Version: "1.0.0",
		},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:           "test-control",
					Name:         "Test Control",
					ObservationDefinitions: []entities.ObservationDefinition{},
				},
			},
		},
	}

	validator := NewProfileValidator()
	err := validator.Validate(profile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one observation is required")
}

func TestValidate_MissingPlugin(t *testing.T) {
	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "test-profile",
			Version: "1.0.0",
		},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "test-control",
					Name: "Test Control",
					ObservationDefinitions: []entities.ObservationDefinition{
						{
							Config: map[string]interface{}{
								"path": "/etc/test",
							},
						},
					},
				},
			},
		},
	}

	validator := NewProfileValidator()
	err := validator.Validate(profile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plugin name is required")
}

// Additional test to verify Validate manually on constructed struct (skipping loader)
func TestValidate_ManualStruct(t *testing.T) {
	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{
			Name:    "manual",
			Version: "1.0.0",
		},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "manual-1",
					Name: "Manual Control",
					ObservationDefinitions: []entities.ObservationDefinition{
						{
							Plugin: "file",
							Config: map[string]interface{}{"path": "/"},
						},
					},
				},
			},
		},
	}

	validator := NewProfileValidator()
	assert.NoError(t, validator.Validate(profile))
}
