package values

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewSeverity(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Severity
		wantErr bool
	}{
		{"low", "low", SevLow, false},
		{"medium", "medium", SevMedium, false},
		{"high", "high", SevHigh, false},
		{"critical", "critical", SevCritical, false},
		{"uppercase", "HIGH", SevHigh, false},
		{"whitespace", "  medium  ", SevMedium, false},
		{"empty", "", SevUnknown, false},
		{"invalid", "invalid", Severity{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sev, err := NewSeverity(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.True(t, sev.Equals(tt.want))
			}
		})
	}
}

func Test_Severity_String(t *testing.T) {
	tests := []struct {
		severity Severity
		expected string
	}{
		{SevLow, "low"},
		{SevMedium, "medium"},
		{SevHigh, "high"},
		{SevCritical, "critical"},
		{SevUnknown, ""},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.severity.String())
		})
	}
}

func Test_Severity_Comparison(t *testing.T) {
	tests := []struct {
		name     string
		sev1     Severity
		sev2     Severity
		isHigher bool
		isEqual  bool
	}{
		{"critical > high", SevCritical, SevHigh, true, false},
		{"high > medium", SevHigh, SevMedium, true, false},
		{"medium > low", SevMedium, SevLow, true, false},
		{"low == low", SevLow, SevLow, false, true},
		{"low < medium", SevLow, SevMedium, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.isHigher, tt.sev1.IsHigherThan(tt.sev2))
			assert.Equal(t, tt.isEqual, tt.sev1.Equals(tt.sev2))

			if tt.isHigher || tt.isEqual {
				assert.True(t, tt.sev1.IsHigherOrEqual(tt.sev2))
			}
		})
	}
}

func Test_Severity_JSON(t *testing.T) {
	original := SevHigh

	data, err := json.Marshal(original)
	require.NoError(t, err)
	assert.Equal(t, `"high"`, string(data))

	var decoded Severity
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.True(t, original.Equals(decoded))
}

func Test_Severity_Scan(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected Severity
		wantErr  bool
	}{
		{"string low", "low", SevLow, false},
		{"string critical", "critical", SevCritical, false},
		{"bytes", []byte("medium"), SevMedium, false},
		{"nil", nil, SevUnknown, false},
		{"invalid type", 123, Severity{}, true},
		{"invalid value", "invalid", Severity{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sev Severity
			err := sev.Scan(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.True(t, sev.Equals(tt.expected))
			}
		})
	}
}
