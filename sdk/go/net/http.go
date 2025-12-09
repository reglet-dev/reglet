//go:build wasip1

package net

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/whiskeyjimbo/reglet/sdk/internal/abi"
	_ "github.com/whiskeyjimbo/reglet/sdk/log" // Initialize WASM logging handler
	"github.com/whiskeyjimbo/reglet/wireformat"
)

// MaxHTTPBodySize is the maximum size of HTTP response body that can be returned.
// Response bodies exceeding this limit will result in an error.
const MaxHTTPBodySize = 10 * 1024 * 1024 // 10 MB

// Define the host function signature for HTTP requests.
// This matches the signature defined in internal/wasm/hostfuncs/registry.go.
//go:wasmimport reglet_host http_request
func host_http_request(requestPacked uint64) uint64

// WasmTransport implements http.RoundTripper for the WASM environment.
// It intercepts standard library HTTP calls and routes them through the host function.
type WasmTransport struct{}

// RoundTrip implements the http.RoundTripper interface.
func (t *WasmTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Create ContextWireFormat from req.Context()
	wireCtx := createContextWireFormat(req.Context())

	// Prepare HTTPRequestWire
	request := HTTPRequestWire{
		Context: wireCtx,
		Method:  req.Method,
		URL:     req.URL.String(),
		Headers: req.Header,
	}

	// Read request body, encode if present
	if req.Body != nil && req.Body != http.NoBody {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("sdk: failed to read request body: %w", err)
		}
		request.Body = base64.StdEncoding.EncodeToString(bodyBytes)
	}

	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("sdk: failed to marshal HTTP request: %w", err)
	}

	// Call the host function
	responsePacked := host_http_request(abi.PtrFromBytes(requestBytes))

	// Read and unmarshal the response
	responseBytes := abi.BytesFromPtr(responsePacked)
	abi.DeallocatePacked(responsePacked) // Free memory on Guest side

	var response HTTPResponseWire
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		return nil, fmt.Errorf("sdk: failed to unmarshal HTTP response: %w", err)
	}

	if response.Error != nil {
		return nil, response.Error // Convert structured error to Go error
	}

	// Check if response body was truncated due to size limit
	// Return explicit error instead of silently truncating
	if response.BodyTruncated {
		return nil, fmt.Errorf("sdk: HTTP response body exceeds maximum size (%d bytes). URL: %s", MaxHTTPBodySize, req.URL.String())
	}

	// Prepare native http.Response
	resp := &http.Response{
		StatusCode: response.StatusCode,
		Header:     response.Headers,
		Request:    req,
		Proto:      "HTTP/1.1", // Default to 1.1
		ProtoMajor: 1,
		ProtoMinor: 1,
		Status:     http.StatusText(response.StatusCode),
	}

	// Decode response body if present
	if response.Body != "" {
		decodedBody, err := base64.StdEncoding.DecodeString(response.Body)
		if err != nil {
			return nil, fmt.Errorf("sdk: failed to decode response body: %w", err)
		}
		resp.Body = io.NopCloser(bytes.NewReader(decodedBody))
		resp.ContentLength = int64(len(decodedBody))
	} else {
		resp.Body = io.NopCloser(bytes.NewReader(nil))
	}

	return resp, nil
}

// REMOVED: init() function that set http.DefaultTransport = &WasmTransport{}
//
// BREAKING CHANGE: Plugins must now explicitly use WasmTransport or SDK helper functions.
//
// Option 1 - Use SDK helper functions (recommended):
//     import sdknet "github.com/whiskeyjimbo/reglet/sdk/net"
//     resp, err := sdknet.Get(ctx, "https://example.com")
//
// Option 2 - Create custom http.Client:
//     client := &http.Client{Transport: &net.WasmTransport{}}
//     resp, err := client.Get("https://example.com")
//
// This change makes HTTP transport configuration explicit instead of implicit,
// avoiding global state mutation and making test isolation easier.

// defaultClient is a reusable HTTP client with WasmTransport.
// Using a single client instance is more efficient than creating a new one for each request.
var defaultClient = &http.Client{
	Transport: &WasmTransport{},
}

// Get is a convenience function for making HTTP GET requests using WasmTransport.
// It's equivalent to http.Get() but uses the WASM host function.
//
// Example:
//     resp, err := net.Get(ctx, "https://api.example.com/status")
//     if err != nil {
//         return err
//     }
//     defer resp.Body.Close()
func Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return defaultClient.Do(req)
}

// Post is a convenience function for making HTTP POST requests using WasmTransport.
//
// Example:
//     body := bytes.NewReader([]byte(`{"key":"value"}`))
//     resp, err := net.Post(ctx, "https://api.example.com/data", "application/json", body)
func Post(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return defaultClient.Do(req)
}

// Do executes an HTTP request using WasmTransport.
// This is useful when you need full control over the request.
//
// Example:
//     req, _ := http.NewRequestWithContext(ctx, "PUT", url, body)
//     req.Header.Set("Authorization", "Bearer "+token)
//     resp, err := net.Do(req)
func Do(req *http.Request) (*http.Response, error) {
	return defaultClient.Do(req)
}

// Re-export HTTP wire format types from shared wireformat package
type (
	HTTPRequestWire  = wireformat.HTTPRequestWire
	HTTPResponseWire = wireformat.HTTPResponseWire
)
