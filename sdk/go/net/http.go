//go:build wasip1

package net

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/whiskeyjimbo/reglet/sdk/internal/abi"
	"github.com/whiskeyjimbo/reglet/wireformat"
)

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

	// Add header if body was truncated
	// This allows plugins to detect incomplete responses
	if response.BodyTruncated {
		if resp.Header == nil {
			resp.Header = make(http.Header)
		}
		resp.Header.Set("X-Reglet-Body-Truncated", "true")
		slog.Warn("SDK: HTTP response body was truncated by host (exceeded 10MB limit)", "url", req.URL.String())
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

// init configures the default HTTP transport to use our WasmTransport.
// This ensures that http.Get(), http.Post(), and other functions that use
// the default transport will use our WASM-aware implementation.
func init() {
	http.DefaultTransport = &WasmTransport{}
	slog.Info("Reglet SDK: HTTP transport initialized.")
}

// Re-export HTTP wire format types from shared wireformat package
type (
	HTTPRequestWire  = wireformat.HTTPRequestWire
	HTTPResponseWire = wireformat.HTTPResponseWire
)
