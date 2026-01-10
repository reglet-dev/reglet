package output

import (
	"bytes"
	"testing"

	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatterFactory_Create(t *testing.T) {
	factory := NewFormatterFactory()
	buf := &bytes.Buffer{}

	tests := []struct {
		name        string
		format      string
		options     ports.FormatterOptions
		wantErr     bool
		wantType    interface{}
		errContains string
	}{
		{
			name:     "table format",
			format:   "table",
			wantType: &TableFormatter{},
		},
		{
			name:     "json format",
			format:   "json",
			options:  ports.FormatterOptions{Indent: true},
			wantType: &JSONFormatter{},
		},
		{
			name:     "yaml format",
			format:   "yaml",
			wantType: &YAMLFormatter{},
		},
		{
			name:     "junit format",
			format:   "junit",
			wantType: &JUnitFormatter{},
		},
		{
			name:     "sarif format",
			format:   "sarif",
			options:  ports.FormatterOptions{ProfilePath: "test.yaml"},
			wantType: &SARIFFormatter{},
		},
		{
			name:        "unknown format",
			format:      "invalid",
			wantErr:     true,
			errContains: "unknown format: invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter, err := factory.Create(tt.format, buf, tt.options)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, formatter)
			assert.IsType(t, tt.wantType, formatter)
		})
	}
}

func TestFormatterFactory_SupportedFormats(t *testing.T) {
	factory := NewFormatterFactory()
	formats := factory.SupportedFormats()

	assert.Contains(t, formats, "table")
	assert.Contains(t, formats, "json")
	assert.Contains(t, formats, "yaml")
	assert.Contains(t, formats, "junit")
	assert.Contains(t, formats, "sarif")
	assert.Len(t, formats, 5)
}
