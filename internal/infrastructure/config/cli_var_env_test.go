package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadValueFromEnv(t *testing.T) {
	tests := []struct {
		name      string
		envVar    string
		envValue  string
		setEnv    bool
		wantError bool
		errMsg    string
	}{
		{
			name:     "env var exists",
			envVar:   "TEST_CLI_VAR_VALUE",
			envValue: "secret-value",
			setEnv:   true,
		},
		{
			name:     "empty env var value",
			envVar:   "TEST_CLI_VAR_EMPTY",
			envValue: "",
			setEnv:   true,
		},
		{
			name:      "env var not set",
			envVar:    "TEST_CLI_VAR_UNDEFINED",
			setEnv:    false,
			wantError: true,
			errMsg:    "environment variable not set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			if tt.setEnv {
				t.Setenv(tt.envVar, tt.envValue)
			} else {
				_ = os.Unsetenv(tt.envVar)
			}

			got, err := ReadValueFromEnv(tt.envVar)

			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.envValue, got)
		})
	}
}

func TestParseSetEnv(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		envSetup  map[string]string
		wantKey   string
		wantValue interface{}
		wantError bool
		errMsg    string
	}{
		{
			name:      "valid key=ENV_VAR",
			input:     "api_key=API_KEY_VAR",
			envSetup:  map[string]string{"API_KEY_VAR": "secret-123"},
			wantKey:   "api_key",
			wantValue: "secret-123",
		},
		{
			name:      "no equals sign",
			input:     "api_key",
			wantError: true,
			errMsg:    "invalid --set-env format",
		},
		{
			name:      "invalid key",
			input:     "123key=ENV_VAR",
			wantError: true,
			errMsg:    "invalid key",
		},
		{
			name:      "env var not set",
			input:     "key=UNDEFINED_VAR",
			wantError: true,
			errMsg:    "environment variable not set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			for k, v := range tt.envSetup {
				t.Setenv(k, v)
			}

			key, value, err := ParseSetEnv(tt.input)

			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantKey, key)
			assert.Equal(t, tt.wantValue, value)
		})
	}
}
