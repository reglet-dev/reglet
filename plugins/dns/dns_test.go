package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/reglet-dev/reglet-sdk/go/application/config"
	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock DNSResolver
type mockDNSResolver struct {
	LookupHostFunc  func(ctx context.Context, host string) ([]string, error)
	LookupMXFunc    func(ctx context.Context, host string) ([]ports.MXRecord, error)
	LookupCNAMEFunc func(ctx context.Context, host string) (string, error)
	LookupTXTFunc   func(ctx context.Context, host string) ([]string, error)
	LookupNSFunc    func(ctx context.Context, host string) ([]string, error)
}

func (m *mockDNSResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	if m.LookupHostFunc != nil {
		return m.LookupHostFunc(ctx, host)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockDNSResolver) LookupMX(ctx context.Context, host string) ([]ports.MXRecord, error) {
	if m.LookupMXFunc != nil {
		return m.LookupMXFunc(ctx, host)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockDNSResolver) LookupCNAME(ctx context.Context, host string) (string, error) {
	if m.LookupCNAMEFunc != nil {
		return m.LookupCNAMEFunc(ctx, host)
	}
	return "", fmt.Errorf("not implemented")
}

func (m *mockDNSResolver) LookupTXT(ctx context.Context, host string) ([]string, error) {
	if m.LookupTXTFunc != nil {
		return m.LookupTXTFunc(ctx, host)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockDNSResolver) LookupNS(ctx context.Context, host string) ([]string, error) {
	if m.LookupNSFunc != nil {
		return m.LookupNSFunc(ctx, host)
	}
	return nil, fmt.Errorf("not implemented")
}

func TestDNSPlugin_Check_ConfigValidation(t *testing.T) {
	// Config validation tests don't necessarily reach the network layer if valid,
	// but if they are valid, they try to execute lookup.
	// We need a mock resolver that returns minimal success to avoid panics.

	mockResolver := &mockDNSResolver{
		LookupHostFunc: func(ctx context.Context, host string) ([]string, error) {
			return []string{"1.2.3.4"}, nil // Default A record success
		},
		LookupMXFunc: func(ctx context.Context, host string) ([]ports.MXRecord, error) {
			return []ports.MXRecord{{Host: "mail.example.com", Pref: 10}}, nil
		},
	}

	plugin := &dnsPlugin{resolver: mockResolver}
	ctx := context.Background()

	tests := []struct {
		name      string
		config    config.Config
		wantError bool
		errMsg    string // Expected part of error message for invalid configs
	}{
		{
			name: "Valid A record config",
			config: config.Config{
				"hostname":    "example.com",
				"record_type": "A",
			},
			wantError: false,
		},
		{
			name: "Valid MX record config",
			config: config.Config{
				"hostname":    "gmail.com",
				"record_type": "MX",
			},
			wantError: false,
		},
		{
			name: "Valid config with nameserver",
			config: config.Config{
				"hostname":    "example.com",
				"record_type": "A",
				"nameserver":  "8.8.8.8:53",
			},
			wantError: false,
		},
		{
			name: "Missing hostname",
			config: config.Config{
				"record_type": "A",
			},
			wantError: true,
			errMsg:    "validation failed for field 'hostname'",
		},
		{
			name: "Invalid record type",
			config: config.Config{
				"hostname":    "example.com",
				"record_type": "INVALID",
			},
			wantError: true,
			errMsg:    "unsupported record type",
		},
		{
			name:      "Empty config (missing hostname)",
			config:    config.Config{},
			wantError: true,
			errMsg:    "validation failed for field 'hostname'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evidence, err := plugin.Check(ctx, tt.config)
			require.NoError(t, err, "Check should not return a Go error directly, but evidence with status/error info")

			if tt.wantError {
				assert.True(t, evidence.IsFailure() || evidence.IsError(), "Expected status failure or error for config error")
				require.NotNil(t, evidence.Error, "Expected evidence to contain an error")
				assert.Contains(t, evidence.Error.Message, tt.errMsg)
				// Allow both config and network errors (validation vs execution failure)
				assert.Contains(t, []string{"config", "network"}, evidence.Error.Type)
			} else {
				// For valid configs, we expect status success now that we have a mock
				assert.True(t, evidence.IsSuccess(), "Expected success for valid config backed by mock")
			}
		})
	}
}
