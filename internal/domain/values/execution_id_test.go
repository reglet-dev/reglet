package valueobjects

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewExecutionID(t *testing.T) {
	id1 := NewExecutionID()
	id2 := NewExecutionID()

	assert.False(t, id1.IsZero(), "new ID should not be zero")
	assert.False(t, id2.IsZero(), "new ID should not be zero")
	assert.False(t, id1.Equals(id2), "two new IDs should be different")
}

func Test_ParseExecutionID(t *testing.T) {
	validUUID := "123e4567-e89b-12d3-a456-426614174000"

	id, err := ParseExecutionID(validUUID)
	require.NoError(t, err)
	assert.Equal(t, validUUID, id.String())
}

func Test_ParseExecutionID_Invalid(t *testing.T) {
	tests := []string{
		"",
		"invalid",
		"123",
		"not-a-uuid",
	}

	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			_, err := ParseExecutionID(tt)
			assert.Error(t, err)
		})
	}
}

func Test_MustParseExecutionID(t *testing.T) {
	validUUID := "123e4567-e89b-12d3-a456-426614174000"
	id := MustParseExecutionID(validUUID)
	assert.Equal(t, validUUID, id.String())
}

func Test_MustParseExecutionID_Panics(t *testing.T) {
	assert.Panics(t, func() {
		MustParseExecutionID("invalid")
	})
}

func Test_FromUUID(t *testing.T) {
	original := uuid.New()
	id := FromUUID(original)
	assert.Equal(t, original, id.UUID())
}

func Test_ExecutionID_IsZero(t *testing.T) {
	zero := ExecutionID{}
	assert.True(t, zero.IsZero())

	nonZero := NewExecutionID()
	assert.False(t, nonZero.IsZero())
}

func Test_ExecutionID_Equals(t *testing.T) {
	id1 := NewExecutionID()
	id2 := NewExecutionID()
	id3 := FromUUID(id1.UUID())

	assert.False(t, id1.Equals(id2))
	assert.True(t, id1.Equals(id3))
}

func Test_ExecutionID_JSON(t *testing.T) {
	original := NewExecutionID()

	// Marshal
	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Unmarshal
	var decoded ExecutionID
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.True(t, original.Equals(decoded))
}

func Test_ExecutionID_JSON_Invalid(t *testing.T) {
	var id ExecutionID
	err := json.Unmarshal([]byte(`"invalid-uuid"`), &id)
	assert.Error(t, err)
}

func Test_ExecutionID_Value(t *testing.T) {
	id := MustParseExecutionID("123e4567-e89b-12d3-a456-426614174000")

	val, err := id.Value()
	require.NoError(t, err)
	assert.Equal(t, "123e4567-e89b-12d3-a456-426614174000", val)
}

func Test_ExecutionID_Scan(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
		wantErr  bool
	}{
		{
			name:     "string",
			input:    "123e4567-e89b-12d3-a456-426614174000",
			expected: "123e4567-e89b-12d3-a456-426614174000",
		},
		{
			name:     "bytes",
			input:    []byte("123e4567-e89b-12d3-a456-426614174000"),
			expected: "123e4567-e89b-12d3-a456-426614174000",
		},
		{
			name:    "nil",
			input:   nil,
			wantErr: false,
		},
		{
			name:    "invalid type",
			input:   123,
			wantErr: true,
		},
		{
			name:    "invalid uuid string",
			input:   "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var id ExecutionID
			err := id.Scan(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.input != nil {
					assert.Equal(t, tt.expected, id.String())
				} else {
					assert.True(t, id.IsZero())
				}
			}
		})
	}
}
