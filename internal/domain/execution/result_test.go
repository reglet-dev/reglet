package execution

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/whiskeyjimbo/reglet/internal/domain/valueobjects"
)

func TestNewExecutionResultWithID(t *testing.T) {
	id := valueobjects.NewExecutionID()
	result := NewExecutionResultWithID(id, "test", "1.0")

	assert.True(t, result.GetID().Equals(id))
	assert.Equal(t, "test", result.ProfileName)
	assert.Equal(t, 1, result.GetVersion())
}

func TestExecutionResult_Version(t *testing.T) {
	result := NewExecutionResult("test", "1.0")
	assert.Equal(t, 1, result.GetVersion())

	result.IncrementVersion()
	assert.Equal(t, 2, result.GetVersion())
}

func TestExecutionResult_IsComplete(t *testing.T) {
	result := NewExecutionResult("test", "1.0")
	assert.False(t, result.IsComplete(1))

	result.AddControlResult(ControlResult{ID: "c1"})
	assert.True(t, result.IsComplete(1))
	assert.False(t, result.IsComplete(2))
}

func TestExecutionResult_GetControlResultByID(t *testing.T) {
	result := NewExecutionResult("test", "1.0")
	result.AddControlResult(ControlResult{ID: "c1", Name: "Control 1"})

	c1 := result.GetControlResultByID("c1")
	assert.NotNil(t, c1)
	assert.Equal(t, "Control 1", c1.Name)

	c2 := result.GetControlResultByID("c2")
	assert.Nil(t, c2)
}

func TestExecutionResult_AddPartialResult(t *testing.T) {
	result := NewExecutionResult("test", "1.0")
	result.AddPartialResult(ControlResult{ID: "c1"})
	
	assert.Len(t, result.Controls, 1)
	assert.Equal(t, "c1", result.Controls[0].ID)
}
