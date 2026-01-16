package engine

import (
	"testing"

	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/pkg/loopexpander"
	"github.com/stretchr/testify/assert"
)

func TestExpandLoopObservation(t *testing.T) {
	obs := entities.ObservationDefinition{
		Plugin: "file",
		Config: map[string]interface{}{
			"path": "{{ .loop.item }}",
		},
		Expect: []string{"data.exists == true"},
		Loop: &entities.LoopConfig{
			Items: "{{ .vars.files }}",
			As:    "",
		},
	}

	loopCtx := &loopexpander.Context{
		Item:   "/etc/passwd",
		Index:  0,
		First:  true,
		Last:   false,
		Length: 3,
	}

	expanded := expandLoopObservation(obs, loopCtx)
	assert.Equal(t, "file", expanded.Plugin)
	assert.Equal(t, "/etc/passwd", expanded.Config["path"])
	assert.Nil(t, expanded.Loop) // Loop should not be copied to expanded child
}
