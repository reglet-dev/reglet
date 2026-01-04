package values

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewControlID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"valid ID", "ctrl-001", "ctrl-001", false},
		{"trims whitespace", "  ctrl-001  ", "ctrl-001", false},
		{"empty string", "", "", true},
		{"whitespace only", "   ", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := NewControlID(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, id.String())
			}
		})
	}
}

func Test_MustNewControlID(t *testing.T) {
	id := MustNewControlID("ctrl-001")
	assert.Equal(t, "ctrl-001", id.String())
}

func Test_MustNewControlID_Panics(t *testing.T) {
	assert.Panics(t, func() {
		MustNewControlID("")
	})
}

func Test_ControlID_IsEmpty(t *testing.T) {
	zero := ControlID{}
	assert.True(t, zero.IsEmpty())

	nonZero := MustNewControlID("ctrl-001")
	assert.False(t, nonZero.IsEmpty())
}

func Test_ControlID_Equals(t *testing.T) {
	id1 := MustNewControlID("ctrl-001")
	id2 := MustNewControlID("ctrl-002")
	id3 := MustNewControlID("ctrl-001")

	assert.False(t, id1.Equals(id2))
	assert.True(t, id1.Equals(id3))
}

func Test_ControlID_JSON(t *testing.T) {
	original := MustNewControlID("ctrl-001")

	data, err := json.Marshal(original)
	require.NoError(t, err)
	assert.Equal(t, `"ctrl-001"`, string(data))

	var decoded ControlID
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.True(t, original.Equals(decoded))
}
