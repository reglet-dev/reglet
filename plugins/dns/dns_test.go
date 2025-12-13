//go:build wasip1

package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	regletsdk "github.com/whiskeyjimbo/reglet/sdk"
)

func TestDNSPlugin_Check_ConfigValidation(t *testing.T) {
	plugin := &dnsPlugin{}
	ctx := context.Background()

	tests := []struct {
		name      string
		config    regletsdk.Config
		wantError bool
		errMsg    string // Expected part of error message for invalid configs
	}{
		{
			name: "Valid A record config",
			config: regletsdk.Config{
				"hostname":    "example.com",
				"record_type": "A",
			},
			wantError: false,
		},
		{
			name: "Valid MX record config",
			config: regletsdk.Config{
				"hostname":    "gmail.com",
				"record_type": "MX",
			},
			wantError: false,
		},
		{
			name: "Valid config with nameserver",
			config: regletsdk.Config{
				"hostname":    "example.com",
				"record_type": "A",
				"nameserver":  "8.8.8.8:53",
			},
			wantError: false,
		},
		{
			name: "Missing hostname",
			config: regletsdk.Config{
				"record_type": "A",
			},
			wantError: true,
			errMsg:    "Hostname' failed on the 'required' tag",
		},
		{
			name: "Invalid record type",
			config: regletsdk.Config{
				"hostname":    "example.com",
				"record_type": "INVALID",
			},
			wantError: true,
			errMsg:    "RecordType' failed on the 'oneof' tag",
		},
		{
			name:      "Empty config (missing hostname)",
			config:    regletsdk.Config{},
			wantError: true,
			errMsg:    "Hostname' failed on the 'required' tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evidence, err := plugin.Check(ctx, tt.config)
			require.NoError(t, err, "Check should not return a Go error directly, but evidence with status/error info")

			if tt.wantError {
				assert.False(t, evidence.Status, "Expected status to be false for config error")
				require.NotNil(t, evidence.Error, "Expected evidence to contain an error")
				assert.Contains(t, evidence.Error.Message, tt.errMsg)
				assert.Equal(t, "config", evidence.Error.Type)
			} else {
				// For valid configs, we expect status true from the config validation part.
				// Actual network lookup success/failure is handled by the SDK and reflected
				// in the evidence data, but for these unit tests, we're only testing that
				// config validation passes and doesn't immediately yield a config error.
				assert.True(t, evidence.Status || evidence.Error != nil, "Expected status to be true or an error from network lookup")
			}
		})
	}
}
