//go:build !wasip1

package net

import (
	"encoding/json"
	"testing"

	"github.com/reglet-dev/reglet/wireformat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: Actual HTTP requests require WASM runtime with host functions.
// These tests focus on wire format structures and data serialization.

func TestHTTPRequestWire_Serialization(t *testing.T) {
	tests := []struct {
		name    string
		request wireformat.HTTPRequestWire
	}{
		{
			name: "GET request",
			request: wireformat.HTTPRequestWire{
				Method: "GET",
				URL:    "https://api.example.com/status",
			},
		},
		{
			name: "POST with body",
			request: wireformat.HTTPRequestWire{
				Method: "POST",
				URL:    "https://api.example.com/data",
				Body:   `{"key":"value"}`,
			},
		},
		{
			name: "request with headers",
			request: wireformat.HTTPRequestWire{
				Method: "GET",
				URL:    "https://api.example.com/data",
				Headers: map[string][]string{
					"Authorization": {"Bearer token123"},
					"Content-Type":  {"application/json"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.request)
			require.NoError(t, err)

			var decoded wireformat.HTTPRequestWire
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.request.Method, decoded.Method)
			assert.Equal(t, tt.request.URL, decoded.URL)
			assert.Equal(t, tt.request.Body, decoded.Body)
			assert.Equal(t, tt.request.Headers, decoded.Headers)
		})
	}
}

func TestHTTPResponseWire_Serialization(t *testing.T) {
	tests := []struct {
		name     string
		response wireformat.HTTPResponseWire
	}{
		{
			name: "successful response",
			response: wireformat.HTTPResponseWire{
				StatusCode: 200,
				Body:       `{"status":"ok"}`,
				Headers: map[string][]string{
					"Content-Type": {"application/json"},
				},
			},
		},
		{
			name: "error response",
			response: wireformat.HTTPResponseWire{
				StatusCode: 404,
				Body:       `{"error":"not found"}`,
			},
		},
		{
			name: "response with error detail",
			response: wireformat.HTTPResponseWire{
				StatusCode: 500,
				Error: &wireformat.ErrorDetail{
					Message: "Internal server error",
					Type:    "network",
					Code:    "500",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.response)
			require.NoError(t, err)

			var decoded wireformat.HTTPResponseWire
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.response.StatusCode, decoded.StatusCode)
			assert.Equal(t, tt.response.Body, decoded.Body)
			assert.Equal(t, tt.response.Headers, decoded.Headers)

			if tt.response.Error != nil {
				require.NotNil(t, decoded.Error)
				assert.Equal(t, tt.response.Error.Message, decoded.Error.Message)
			}
		})
	}
}

func TestHTTPMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := wireformat.HTTPRequestWire{
				Method: method,
				URL:    "https://api.example.com/resource",
			}

			data, err := json.Marshal(req)
			require.NoError(t, err)

			var decoded wireformat.HTTPRequestWire
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)
			assert.Equal(t, method, decoded.Method)
		})
	}
}

func TestHTTPStatusCodes(t *testing.T) {
	codes := []int{200, 201, 204, 301, 302, 400, 401, 403, 404, 500, 502, 503}

	for _, code := range codes {
		t.Run(string(rune(code)), func(t *testing.T) {
			resp := wireformat.HTTPResponseWire{
				StatusCode: code,
				Body:       "response body",
			}

			data, err := json.Marshal(resp)
			require.NoError(t, err)

			var decoded wireformat.HTTPResponseWire
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)
			assert.Equal(t, code, decoded.StatusCode)
		})
	}
}

// Test 10MB truncation constant documentation
func TestHTTPBodySizeLimits(t *testing.T) {
	// Note: Actual truncation happens in http.go with 10MB limit
	// This test documents expected behavior with BodyTruncated flag

	resp := wireformat.HTTPResponseWire{
		StatusCode:    200,
		Body:          "large response body",
		BodyTruncated: true,
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded wireformat.HTTPResponseWire
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.True(t, decoded.BodyTruncated)
}
