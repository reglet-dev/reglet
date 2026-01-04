package values

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewPluginName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"valid", "http", "http", false},
		{"with version", "http@1.0.0", "http@1.0.0", false},
		{"trims whitespace", "  http  ", "http", false},
		{"empty", "", "", true},
		{"whitespace only", "   ", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pn, err := NewPluginName(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, pn.String())
			}
		})
	}
}

func Test_MustNewPluginName(t *testing.T) {
	pn := MustNewPluginName("http")
	assert.Equal(t, "http", pn.String())
}

func Test_MustNewPluginName_Panics(t *testing.T) {
	assert.Panics(t, func() {
		MustNewPluginName("")
	})
}

func Test_PluginName_IsEmpty(t *testing.T) {
	zero := PluginName{}
	assert.True(t, zero.IsEmpty())

	nonZero := MustNewPluginName("http")
	assert.False(t, nonZero.IsEmpty())
}

func Test_PluginName_Equals(t *testing.T) {
	pn1 := MustNewPluginName("http")
	pn2 := MustNewPluginName("tcp")
	pn3 := MustNewPluginName("http")

	assert.False(t, pn1.Equals(pn2))
	assert.True(t, pn1.Equals(pn3))
}

func Test_PluginName_JSON(t *testing.T) {
	original := MustNewPluginName("http")

	data, err := json.Marshal(original)
	require.NoError(t, err)
	assert.Equal(t, `"http"`, string(data))

	var decoded PluginName
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.True(t, original.Equals(decoded))
}
