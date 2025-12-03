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

	"github.com/whiskeyjimbo/reglet/sdk" // For sdk.ErrorDetail
	"github.com/whiskeyjimbo/reglet/sdk/internal/abi"
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

// HTTPRequestWire is the JSON wire format for an HTTP request from Guest to Host.
type HTTPRequestWire struct {
	Context ContextWireFormat `json:"context"`
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string][]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"` // Base64 encoded for binary, or plain string
}

// HTTPResponseWire is the JSON wire format for an HTTP response from Host to Guest.
type HTTPResponseWire struct {
	StatusCode int               `json:"status_code"`
	Headers    map[string][]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"` // Base64 encoded for binary, or plain string
	Error      *sdk.ErrorDetail  `json:"error,omitempty"` // Structured error
}
