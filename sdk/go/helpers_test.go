package sdk_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "github.com/whiskeyjimbo/reglet/sdk"
)

func TestGetString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  sdk.Config
		key     string
		wantVal string
		wantOK  bool
	}{
		{
			name:    "string value found",
			config:  sdk.Config{"hostname": "example.com"},
			key:     "hostname",
			wantVal: "example.com",
			wantOK:  true,
		},
		{
			name:    "key not found",
			config:  sdk.Config{"other": "value"},
			key:     "hostname",
			wantVal: "",
			wantOK:  false,
		},
		{
			name:    "wrong type",
			config:  sdk.Config{"hostname": 123},
			key:     "hostname",
			wantVal: "",
			wantOK:  false,
		},
		{
			name:    "nil config",
			config:  nil,
			key:     "hostname",
			wantVal: "",
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			val, ok := sdk.GetString(tt.config, tt.key)
			assert.Equal(t, tt.wantVal, val)
			assert.Equal(t, tt.wantOK, ok)
		})
	}
}

func TestGetInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  sdk.Config
		key     string
		wantVal int
		wantOK  bool
	}{
		{
			name:    "int value",
			config:  sdk.Config{"port": 443},
			key:     "port",
			wantVal: 443,
			wantOK:  true,
		},
		{
			name:    "float64 value (JSON default)",
			config:  sdk.Config{"port": float64(443)},
			key:     "port",
			wantVal: 443,
			wantOK:  true,
		},
		{
			name:    "int64 value",
			config:  sdk.Config{"port": int64(443)},
			key:     "port",
			wantVal: 443,
			wantOK:  true,
		},
		{
			name:    "string value - wrong type",
			config:  sdk.Config{"port": "443"},
			key:     "port",
			wantVal: 0,
			wantOK:  false,
		},
		{
			name:    "key not found",
			config:  sdk.Config{},
			key:     "port",
			wantVal: 0,
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			val, ok := sdk.GetInt(tt.config, tt.key)
			assert.Equal(t, tt.wantVal, val)
			assert.Equal(t, tt.wantOK, ok)
		})
	}
}

func TestGetBool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  sdk.Config
		key     string
		wantVal bool
		wantOK  bool
	}{
		{
			name:    "true value",
			config:  sdk.Config{"enabled": true},
			key:     "enabled",
			wantVal: true,
			wantOK:  true,
		},
		{
			name:    "false value",
			config:  sdk.Config{"enabled": false},
			key:     "enabled",
			wantVal: false,
			wantOK:  true,
		},
		{
			name:    "string value - wrong type",
			config:  sdk.Config{"enabled": "true"},
			key:     "enabled",
			wantVal: false,
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			val, ok := sdk.GetBool(tt.config, tt.key)
			assert.Equal(t, tt.wantVal, val)
			assert.Equal(t, tt.wantOK, ok)
		})
	}
}

func TestGetStringSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  sdk.Config
		key     string
		wantVal []string
		wantOK  bool
	}{
		{
			name:    "valid string slice",
			config:  sdk.Config{"tags": []interface{}{"a", "b", "c"}},
			key:     "tags",
			wantVal: []string{"a", "b", "c"},
			wantOK:  true,
		},
		{
			name:    "empty slice",
			config:  sdk.Config{"tags": []interface{}{}},
			key:     "tags",
			wantVal: []string{},
			wantOK:  true,
		},
		{
			name:    "mixed types - fails",
			config:  sdk.Config{"tags": []interface{}{"a", 123}},
			key:     "tags",
			wantVal: nil,
			wantOK:  false,
		},
		{
			name:    "key not found",
			config:  sdk.Config{},
			key:     "tags",
			wantVal: nil,
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			val, ok := sdk.GetStringSlice(tt.config, tt.key)
			assert.Equal(t, tt.wantVal, val)
			assert.Equal(t, tt.wantOK, ok)
		})
	}
}

func TestMustGetString(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		config := sdk.Config{"hostname": "example.com"}
		val, err := sdk.MustGetString(config, "hostname")
		require.NoError(t, err)
		assert.Equal(t, "example.com", val)
	})

	t.Run("missing key", func(t *testing.T) {
		t.Parallel()
		config := sdk.Config{}
		_, err := sdk.MustGetString(config, "hostname")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "hostname")
	})

	t.Run("wrong type", func(t *testing.T) {
		t.Parallel()
		config := sdk.Config{"hostname": 123}
		_, err := sdk.MustGetString(config, "hostname")
		require.Error(t, err)
	})
}

func TestGetStringDefault(t *testing.T) {
	t.Parallel()

	t.Run("value exists", func(t *testing.T) {
		t.Parallel()
		config := sdk.Config{"hostname": "custom.com"}
		val := sdk.GetStringDefault(config, "hostname", "default.com")
		assert.Equal(t, "custom.com", val)
	})

	t.Run("uses default", func(t *testing.T) {
		t.Parallel()
		config := sdk.Config{}
		val := sdk.GetStringDefault(config, "hostname", "default.com")
		assert.Equal(t, "default.com", val)
	})
}

func TestGetIntDefault(t *testing.T) {
	t.Parallel()

	t.Run("value exists", func(t *testing.T) {
		t.Parallel()
		config := sdk.Config{"port": 8080}
		val := sdk.GetIntDefault(config, "port", 443)
		assert.Equal(t, 8080, val)
	})

	t.Run("uses default", func(t *testing.T) {
		t.Parallel()
		config := sdk.Config{}
		val := sdk.GetIntDefault(config, "port", 443)
		assert.Equal(t, 443, val)
	})
}
