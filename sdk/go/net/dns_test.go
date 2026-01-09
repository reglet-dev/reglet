//go:build !wasip1

package net

import (
	"encoding/json"
	"testing"

	"github.com/reglet-dev/reglet/wireformat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: Actual DNS lookups require WASM runtime with host functions.
// These tests focus on data structures and wire format serialization.
// The WasmResolver type is only available in WASM builds (wasip1 tag).

func TestDNSRequestWire_Serialization(t *testing.T) {
	tests := []struct {
		name       string
		request    DNSRequestWire
		wantFields map[string]interface{}
	}{
		{
			name: "A record query",
			request: DNSRequestWire{
				Hostname: "example.com",
				Type:     "A",
			},
			wantFields: map[string]interface{}{
				"hostname": "example.com",
				"type":     "A",
			},
		},
		{
			name: "AAAA record query",
			request: DNSRequestWire{
				Hostname: "example.com",
				Type:     "AAAA",
			},
			wantFields: map[string]interface{}{
				"hostname": "example.com",
				"type":     "AAAA",
			},
		},
		{
			name: "MX record query",
			request: DNSRequestWire{
				Hostname: "example.com",
				Type:     "MX",
			},
			wantFields: map[string]interface{}{
				"hostname": "example.com",
				"type":     "MX",
			},
		},
		{
			name: "query with custom nameserver",
			request: DNSRequestWire{
				Hostname:   "example.com",
				Type:       "A",
				Nameserver: "8.8.8.8:53",
			},
			wantFields: map[string]interface{}{
				"hostname":   "example.com",
				"type":       "A",
				"nameserver": "8.8.8.8:53",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.request)
			require.NoError(t, err)

			// Test JSON unmarshaling
			var decoded DNSRequestWire
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			// Verify fields
			assert.Equal(t, tt.request.Hostname, decoded.Hostname)
			assert.Equal(t, tt.request.Type, decoded.Type)
			assert.Equal(t, tt.request.Nameserver, decoded.Nameserver)
		})
	}
}

func TestDNSResponseWire_Serialization(t *testing.T) {
	tests := []struct {
		name     string
		response DNSResponseWire
	}{
		{
			name: "successful A record response",
			response: DNSResponseWire{
				Records: []string{"93.184.216.34"},
			},
		},
		{
			name: "multiple A records",
			response: DNSResponseWire{
				Records: []string{"93.184.216.34", "93.184.216.35"},
			},
		},
		{
			name: "MX records",
			response: DNSResponseWire{
				Records: []string{
					"10 mail1.example.com",
					"20 mail2.example.com",
				},
			},
		},
		{
			name: "TXT records",
			response: DNSResponseWire{
				Records: []string{
					"v=spf1 include:_spf.example.com ~all",
				},
			},
		},
		{
			name: "error response",
			response: DNSResponseWire{
				Error: &wireformat.ErrorDetail{
					Message: "DNS query failed",
					Type:    "network",
					Code:    "NXDOMAIN",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.response)
			require.NoError(t, err)

			// Test JSON unmarshaling
			var decoded DNSResponseWire
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			// Verify records
			assert.Equal(t, tt.response.Records, decoded.Records)

			// Verify error if present
			if tt.response.Error != nil {
				require.NotNil(t, decoded.Error)
				assert.Equal(t, tt.response.Error.Message, decoded.Error.Message)
				assert.Equal(t, tt.response.Error.Type, decoded.Error.Type)
				assert.Equal(t, tt.response.Error.Code, decoded.Error.Code)
			} else {
				assert.Nil(t, decoded.Error)
			}
		})
	}
}

func TestDNSRecordTypes(t *testing.T) {
	// Test that all common DNS record types can be represented
	recordTypes := []string{
		"A",
		"AAAA",
		"CNAME",
		"MX",
		"NS",
		"TXT",
		"SOA",
		"PTR",
		"SRV",
	}

	for _, recordType := range recordTypes {
		t.Run(recordType, func(t *testing.T) {
			req := DNSRequestWire{
				Hostname: "example.com",
				Type:     recordType,
			}

			data, err := json.Marshal(req)
			require.NoError(t, err)

			var decoded DNSRequestWire
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)
			assert.Equal(t, recordType, decoded.Type)
		})
	}
}

func TestDNSWireFormat_EmptyRecords(t *testing.T) {
	// Test that empty record responses are handled correctly
	response := DNSResponseWire{
		Records: []string{},
	}

	data, err := json.Marshal(response)
	require.NoError(t, err)

	var decoded DNSResponseWire
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// JSON unmarshaling of empty/omitted array fields results in nil slice (Go behavior)
	// This is acceptable - both nil and empty slice represent "no records"
	assert.Len(t, decoded.Records, 0)
}

func TestDNSWireFormat_ErrorHandling(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		errorCode string
		message   string
	}{
		{
			name:      "NXDOMAIN",
			errorType: "network",
			errorCode: "NXDOMAIN",
			message:   "domain does not exist",
		},
		{
			name:      "timeout",
			errorType: "timeout",
			errorCode: "ETIMEDOUT",
			message:   "DNS query timed out",
		},
		{
			name:      "server failure",
			errorType: "network",
			errorCode: "SERVFAIL",
			message:   "DNS server returned SERVFAIL",
		},
		{
			name:      "refused",
			errorType: "network",
			errorCode: "REFUSED",
			message:   "DNS server refused query",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := DNSResponseWire{
				Error: &wireformat.ErrorDetail{
					Type:    tt.errorType,
					Code:    tt.errorCode,
					Message: tt.message,
				},
			}

			data, err := json.Marshal(response)
			require.NoError(t, err)

			var decoded DNSResponseWire
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			require.NotNil(t, decoded.Error)
			assert.Equal(t, tt.errorType, decoded.Error.Type)
			assert.Equal(t, tt.errorCode, decoded.Error.Code)
			assert.Equal(t, tt.message, decoded.Error.Message)
		})
	}
}

// Integration test notes for WASM environment (requires wasip1 build):
// The following tests would be added in a separate dns_integration_test.go file:
//
// - TestWasmResolver_LookupHost: Test with known domains (example.com)
// - TestWasmResolver_LookupMX: Test with known mail servers
// - TestWasmResolver_LookupTXT: Test SPF/DKIM records
// - TestWasmResolver_Timeout: Test timeout behavior with slow DNS servers
// - TestWasmResolver_NXDOMAIN: Test error handling for non-existent domains
// - TestWasmResolver_Cancellation: Test cancellation via context
// - TestWasmResolver_CustomNameserver: Test custom nameserver configuration
//
// WasmResolver methods (verified to exist in dns.go):
// - LookupHost(ctx, host) ([]string, error)
// - LookupMX(ctx, host) ([]*MXRecord, error)
// - LookupTXT(ctx, host) ([]string, error)
// - LookupNS(ctx, host) ([]string, error)
// - LookupA(ctx, host) ([]string, error)
// - LookupAAAA(ctx, host) ([]string, error)
// - LookupIPAddr(ctx, host) ([]net.IPAddr, error)
