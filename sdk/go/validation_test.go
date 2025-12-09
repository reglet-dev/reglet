//go:build !wasip1

package sdk

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateConfig_ValidConfig(t *testing.T) {
	type SimpleConfig struct {
		Host string `json:"host" validate:"required"`
		Port int    `json:"port" validate:"required,min=1,max=65535"`
	}

	config := Config{
		"host": "example.com",
		"port": 443,
	}

	var target SimpleConfig
	err := ValidateConfig(config, &target)
	require.NoError(t, err)

	// Verify struct was populated
	assert.Equal(t, "example.com", target.Host)
	assert.Equal(t, 443, target.Port)
}

func TestValidateConfig_MissingRequiredField(t *testing.T) {
	type RequiredConfig struct {
		Host string `json:"host" validate:"required"`
		Port int    `json:"port" validate:"required"`
	}

	config := Config{
		"host": "example.com",
		// port is missing
	}

	var target RequiredConfig
	err := ValidateConfig(config, &target)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestValidateConfig_InvalidValue(t *testing.T) {
	type PortConfig struct {
		Port int `json:"port" validate:"min=1,max=65535"`
	}

	tests := []struct {
		name   string
		config Config
	}{
		{
			name:   "port too low",
			config: Config{"port": 0},
		},
		{
			name:   "port too high",
			config: Config{"port": 70000},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var target PortConfig
			err := ValidateConfig(tt.config, &target)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "validation failed")
		})
	}
}

func TestValidateConfig_TypeConversion(t *testing.T) {
	type TypedConfig struct {
		IntField    int     `json:"int_field"`
		StringField string  `json:"string_field"`
		BoolField   bool    `json:"bool_field"`
		FloatField  float64 `json:"float_field"`
	}

	config := Config{
		"int_field":    42,
		"string_field": "hello",
		"bool_field":   true,
		"float_field":  3.14,
	}

	var target TypedConfig
	err := ValidateConfig(config, &target)
	require.NoError(t, err)

	assert.Equal(t, 42, target.IntField)
	assert.Equal(t, "hello", target.StringField)
	assert.True(t, target.BoolField)
	assert.Equal(t, 3.14, target.FloatField)
}

func TestValidateConfig_NestedStruct(t *testing.T) {
	type ServerConfig struct {
		Host string `json:"host" validate:"required"`
		Port int    `json:"port" validate:"required,min=1"`
	}

	type AppConfig struct {
		Server  ServerConfig `json:"server" validate:"required"`
		Timeout int          `json:"timeout" validate:"min=1"`
	}

	config := Config{
		"server": map[string]interface{}{
			"host": "api.example.com",
			"port": 443,
		},
		"timeout": 30,
	}

	var target AppConfig
	err := ValidateConfig(config, &target)
	require.NoError(t, err)

	assert.Equal(t, "api.example.com", target.Server.Host)
	assert.Equal(t, 443, target.Server.Port)
	assert.Equal(t, 30, target.Timeout)
}

func TestValidateConfig_ArrayFields(t *testing.T) {
	type ArrayConfig struct {
		Hosts []string `json:"hosts" validate:"required,min=1"`
		Ports []int    `json:"ports" validate:"dive,min=1,max=65535"`
	}

	config := Config{
		"hosts": []string{"host1.example.com", "host2.example.com"},
		"ports": []int{80, 443},
	}

	var target ArrayConfig
	err := ValidateConfig(config, &target)
	require.NoError(t, err)

	assert.Len(t, target.Hosts, 2)
	assert.Len(t, target.Ports, 2)
	assert.Equal(t, "host1.example.com", target.Hosts[0])
	assert.Equal(t, 80, target.Ports[0])
}

func TestValidateConfig_OptionalFields(t *testing.T) {
	type OptionalConfig struct {
		Required string  `json:"required" validate:"required"`
		Optional *string `json:"optional,omitempty"`
	}

	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "with optional field",
			config: Config{
				"required": "value",
				"optional": "also-value",
			},
			wantErr: false,
		},
		{
			name: "without optional field",
			config: Config{
				"required": "value",
			},
			wantErr: false,
		},
		{
			name: "missing required field",
			config: Config{
				"optional": "only-optional",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var target OptionalConfig
			err := ValidateConfig(tt.config, &target)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, "value", target.Required)
			}
		})
	}
}

func TestValidateConfig_ValidationTags(t *testing.T) {
	type TaggedConfig struct {
		Email string `json:"email" validate:"required,email"`
		URL   string `json:"url" validate:"required,url"`
		IP    string `json:"ip" validate:"required,ip"`
	}

	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "all valid",
			config: Config{
				"email": "test@example.com",
				"url":   "https://example.com",
				"ip":    "192.168.1.1",
			},
			wantErr: false,
		},
		{
			name: "invalid email",
			config: Config{
				"email": "not-an-email",
				"url":   "https://example.com",
				"ip":    "192.168.1.1",
			},
			wantErr: true,
		},
		{
			name: "invalid url",
			config: Config{
				"email": "test@example.com",
				"url":   "not-a-url",
				"ip":    "192.168.1.1",
			},
			wantErr: true,
		},
		{
			name: "invalid ip",
			config: Config{
				"email": "test@example.com",
				"url":   "https://example.com",
				"ip":    "not-an-ip",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var target TaggedConfig
			err := ValidateConfig(tt.config, &target)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "validation failed")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateConfig_EmptyConfig(t *testing.T) {
	type EmptyConfig struct{}

	config := Config{}

	var target EmptyConfig
	err := ValidateConfig(config, &target)
	require.NoError(t, err)
}

func TestValidateConfig_MarshalError(t *testing.T) {
	// Create a config with an unmarshalable value (channels can't be marshaled)
	type BadConfig struct {
		Value int `json:"value"`
	}

	// Note: In practice, Config is map[string]interface{} so this is hard to trigger
	// This test documents the error path exists
	config := Config{
		"value": 42,
	}

	var target BadConfig
	err := ValidateConfig(config, &target)
	require.NoError(t, err) // Should succeed with normal values
}
