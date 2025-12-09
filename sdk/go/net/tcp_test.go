//go:build !wasip1

package net

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/whiskeyjimbo/reglet/wireformat"
)

// Note: Actual TCP connections require WASM runtime with host functions.
// These tests focus on wire format structures and data serialization.

func TestTCPRequestWire_Serialization(t *testing.T) {
	tests := []struct {
		name    string
		request TCPRequestWire
	}{
		{
			name: "basic TCP connection",
			request: TCPRequestWire{
				Host: "example.com",
				Port: "443",
			},
		},
		{
			name: "TLS connection",
			request: TCPRequestWire{
				Host: "api.example.com",
				Port: "443",
				TLS:  true,
			},
		},
		{
			name: "connection with timeout",
			request: TCPRequestWire{
				Host:      "slow-server.example.com",
				Port:      "80",
				TimeoutMs: 10000,
			},
		},
		{
			name: "custom port",
			request: TCPRequestWire{
				Host: "localhost",
				Port: "8080",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.request)
			require.NoError(t, err)

			var decoded TCPRequestWire
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.request.Host, decoded.Host)
			assert.Equal(t, tt.request.Port, decoded.Port)
			assert.Equal(t, tt.request.TLS, decoded.TLS)
			assert.Equal(t, tt.request.TimeoutMs, decoded.TimeoutMs)
		})
	}
}

func TestTCPResponseWire_Serialization(t *testing.T) {
	tests := []struct {
		name     string
		response TCPResponseWire
	}{
		{
			name: "successful connection",
			response: TCPResponseWire{
				Connected: true,
				Address:   "example.com:443",
			},
		},
		{
			name: "failed connection",
			response: TCPResponseWire{
				Connected: false,
				Address:   "unreachable.example.com:443",
				Error: &wireformat.ErrorDetail{
					Message: "connection refused",
					Type:    "network",
					Code:    "ECONNREFUSED",
				},
			},
		},
		{
			name: "TLS handshake success",
			response: TCPResponseWire{
				Connected: true,
				Address:   "secure.example.com:443",
				TLS:       true,
			},
		},
		{
			name: "timeout error",
			response: TCPResponseWire{
				Connected: false,
				Address:   "slow.example.com:443",
				Error: &wireformat.ErrorDetail{
					Message: "connection timeout",
					Type:    "timeout",
					Code:    "ETIMEDOUT",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.response)
			require.NoError(t, err)

			var decoded TCPResponseWire
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.response.Connected, decoded.Connected)
			assert.Equal(t, tt.response.Address, decoded.Address)
			assert.Equal(t, tt.response.TLS, decoded.TLS)

			if tt.response.Error != nil {
				require.NotNil(t, decoded.Error)
				assert.Equal(t, tt.response.Error.Message, decoded.Error.Message)
				assert.Equal(t, tt.response.Error.Type, decoded.Error.Type)
			}
		})
	}
}

func TestTCPCommonPorts(t *testing.T) {
	ports := map[string]string{
		"HTTP":  "80",
		"HTTPS": "443",
		"SSH":   "22",
		"FTP":   "21",
		"SMTP":  "25",
		"DNS":   "53",
		"MySQL": "3306",
		"Redis": "6379",
	}

	for service, port := range ports {
		t.Run(service, func(t *testing.T) {
			req := TCPRequestWire{
				Host: "example.com",
				Port: port,
			}

			data, err := json.Marshal(req)
			require.NoError(t, err)

			var decoded TCPRequestWire
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)
			assert.Equal(t, port, decoded.Port)
		})
	}
}

func TestTCPErrorTypes(t *testing.T) {
	errors := []struct {
		code    string
		errType string
		message string
	}{
		{"ECONNREFUSED", "network", "connection refused"},
		{"ETIMEDOUT", "timeout", "connection timeout"},
		{"EHOSTUNREACH", "network", "host unreachable"},
		{"ENETUNREACH", "network", "network unreachable"},
		{"TLS_ERROR", "tls", "TLS handshake failed"},
	}

	for _, e := range errors {
		t.Run(e.code, func(t *testing.T) {
			resp := TCPResponseWire{
				Connected: false,
				Error: &wireformat.ErrorDetail{
					Code:    e.code,
					Type:    e.errType,
					Message: e.message,
				},
			}

			data, err := json.Marshal(resp)
			require.NoError(t, err)

			var decoded TCPResponseWire
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			require.NotNil(t, decoded.Error)
			assert.Equal(t, e.code, decoded.Error.Code)
			assert.Equal(t, e.errType, decoded.Error.Type)
		})
	}
}
