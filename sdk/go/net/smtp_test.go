//go:build !wasip1

package net

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/whiskeyjimbo/reglet/wireformat"
)

// Note: Actual SMTP connections require WASM runtime with host functions.
// These tests focus on wire format structures and data serialization.

func TestSMTPRequestWire_Serialization(t *testing.T) {
	tests := []struct {
		name    string
		request SMTPRequestWire
	}{
		{
			name: "basic SMTP connection",
			request: SMTPRequestWire{
				Host: "smtp.example.com",
				Port: "25",
			},
		},
		{
			name: "SMTPS connection (TLS)",
			request: SMTPRequestWire{
				Host: "smtp.example.com",
				Port: "465",
				TLS:  true,
			},
		},
		{
			name: "SMTP with STARTTLS",
			request: SMTPRequestWire{
				Host:     "smtp.example.com",
				Port:     "587",
				StartTLS: true,
			},
		},
		{
			name: "connection with timeout",
			request: SMTPRequestWire{
				Host:      "smtp.example.com",
				Port:      "25",
				TimeoutMs: 10000,
			},
		},
		{
			name: "custom port with TLS and STARTTLS",
			request: SMTPRequestWire{
				Host:     "smtp.example.com",
				Port:     "2525",
				TLS:      false,
				StartTLS: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.request)
			require.NoError(t, err)

			var decoded SMTPRequestWire
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.request.Host, decoded.Host)
			assert.Equal(t, tt.request.Port, decoded.Port)
			assert.Equal(t, tt.request.TLS, decoded.TLS)
			assert.Equal(t, tt.request.StartTLS, decoded.StartTLS)
			assert.Equal(t, tt.request.TimeoutMs, decoded.TimeoutMs)
		})
	}
}

func TestSMTPResponseWire_Serialization(t *testing.T) {
	tests := []struct {
		name     string
		response SMTPResponseWire
	}{
		{
			name: "successful connection",
			response: SMTPResponseWire{
				Connected: true,
				Address:   "smtp.example.com:25",
				Banner:    "220 smtp.example.com ESMTP",
			},
		},
		{
			name: "failed connection",
			response: SMTPResponseWire{
				Connected: false,
				Address:   "unreachable.example.com:25",
				Error: &wireformat.ErrorDetail{
					Message: "connection refused",
					Type:    "network",
					Code:    "ECONNREFUSED",
				},
			},
		},
		{
			name: "TLS connection success",
			response: SMTPResponseWire{
				Connected:      true,
				Address:        "smtp.example.com:465",
				Banner:         "220 smtp.example.com ESMTP ready",
				TLS:            true,
				TLSVersion:     "TLS 1.3",
				TLSCipherSuite: "TLS_AES_128_GCM_SHA256",
				TLSServerName:  "smtp.example.com",
			},
		},
		{
			name: "timeout error",
			response: SMTPResponseWire{
				Connected: false,
				Address:   "slow.example.com:25",
				Error: &wireformat.ErrorDetail{
					Message: "connection timeout",
					Type:    "timeout",
					Code:    "ETIMEDOUT",
				},
			},
		},
		{
			name: "STARTTLS upgrade success",
			response: SMTPResponseWire{
				Connected:      true,
				Address:        "smtp.example.com:587",
				Banner:         "220 smtp.example.com ESMTP",
				TLS:            true,
				TLSVersion:     "TLS 1.2",
				TLSCipherSuite: "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.response)
			require.NoError(t, err)

			var decoded SMTPResponseWire
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.response.Connected, decoded.Connected)
			assert.Equal(t, tt.response.Address, decoded.Address)
			assert.Equal(t, tt.response.Banner, decoded.Banner)
			assert.Equal(t, tt.response.TLS, decoded.TLS)
			assert.Equal(t, tt.response.TLSVersion, decoded.TLSVersion)

			if tt.response.Error != nil {
				require.NotNil(t, decoded.Error)
				assert.Equal(t, tt.response.Error.Message, decoded.Error.Message)
				assert.Equal(t, tt.response.Error.Type, decoded.Error.Type)
			}
		})
	}
}

func TestSMTPCommonPorts(t *testing.T) {
	ports := map[string]string{
		"SMTP":            "25",
		"SMTPS":           "465",
		"Submission":      "587",
		"AlternativeSMTP": "2525",
	}

	for service, port := range ports {
		t.Run(service, func(t *testing.T) {
			req := SMTPRequestWire{
				Host: "smtp.example.com",
				Port: port,
			}

			data, err := json.Marshal(req)
			require.NoError(t, err)

			var decoded SMTPRequestWire
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)
			assert.Equal(t, port, decoded.Port)
		})
	}
}

func TestSMTPErrorTypes(t *testing.T) {
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
		{"STARTTLS_ERROR", "tls", "STARTTLS upgrade failed"},
	}

	for _, e := range errors {
		t.Run(e.code, func(t *testing.T) {
			resp := SMTPResponseWire{
				Connected: false,
				Error: &wireformat.ErrorDetail{
					Code:    e.code,
					Type:    e.errType,
					Message: e.message,
				},
			}

			data, err := json.Marshal(resp)
			require.NoError(t, err)

			var decoded SMTPResponseWire
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			require.NotNil(t, decoded.Error)
			assert.Equal(t, e.code, decoded.Error.Code)
			assert.Equal(t, e.errType, decoded.Error.Type)
		})
	}
}

func TestSMTPBannerParsing(t *testing.T) {
	banners := []struct {
		name   string
		banner string
	}{
		{"standard banner", "220 smtp.example.com ESMTP Postfix"},
		{"Microsoft Exchange", "220 mail.example.com Microsoft ESMTP MAIL Service ready"},
		{"Google Mail", "220 smtp.gmail.com ESMTP"},
		{"Amazon SES", "220 email-smtp.us-east-1.amazonaws.com ESMTP"},
		{"minimal banner", "220 ESMTP"},
	}

	for _, tt := range banners {
		t.Run(tt.name, func(t *testing.T) {
			resp := SMTPResponseWire{
				Connected: true,
				Banner:    tt.banner,
			}

			data, err := json.Marshal(resp)
			require.NoError(t, err)

			var decoded SMTPResponseWire
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.banner, decoded.Banner)
		})
	}
}
