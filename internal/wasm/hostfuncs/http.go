package hostfuncs

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tetratelabs/wazero/api"
)

// HTTPRequest performs HTTP/HTTPS requests on behalf of the plugin
//
// Parameters (via WASM stack):
//   - urlPtr, urlLen: URL to request
//   - methodPtr, methodLen: HTTP method (GET, POST, PUT, DELETE, etc.)
//   - headersPtr, headersLen: JSON object of headers (optional, can be 0)
//   - bodyPtr, bodyLen: Request body (optional, can be 0)
//
// Returns: Pointer to JSON result in WASM memory:
//
//	{
//	  "status": true,
//	  "status_code": 200,
//	  "headers": {"Content-Type": ["application/json"]},
//	  "body": "response body",
//	  "response_time_ms": 123
//	}
//
// or error:
//
//	{"status": false, "error": "error message"}
func HTTPRequest(ctx context.Context, mod api.Module, stack []uint64, checker *CapabilityChecker) {
	urlPtr := uint32(stack[0])
	urlLen := uint32(stack[1])
	methodPtr := uint32(stack[2])
	methodLen := uint32(stack[3])
	headersPtr := uint32(stack[4])
	headersLen := uint32(stack[5])
	bodyPtr := uint32(stack[6])
	bodyLen := uint32(stack[7])

	// Check capability for outbound HTTP/HTTPS
	if err := checker.Check("network", "outbound:80,443"); err != nil {
		stack[0] = uint64(writeError(ctx, mod, fmt.Sprintf("permission denied: %v", err)))
		return
	}

	// Read URL from WASM memory
	urlBytes, ok := mod.Memory().Read(urlPtr, urlLen)
	if !ok {
		stack[0] = uint64(writeError(ctx, mod, "failed to read URL from memory"))
		return
	}
	url := string(urlBytes)

	// Read method from WASM memory
	methodBytes, ok := mod.Memory().Read(methodPtr, methodLen)
	if !ok {
		stack[0] = uint64(writeError(ctx, mod, "failed to read method from memory"))
		return
	}
	method := strings.ToUpper(string(methodBytes))

	// Validate method
	validMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "DELETE": true,
		"HEAD": true, "OPTIONS": true, "PATCH": true,
	}
	if !validMethods[method] {
		stack[0] = uint64(writeError(ctx, mod, fmt.Sprintf("invalid HTTP method: %s", method)))
		return
	}

	// Read request body (if provided)
	var body io.Reader
	if bodyLen > 0 {
		bodyBytes, ok := mod.Memory().Read(bodyPtr, bodyLen)
		if !ok {
			stack[0] = uint64(writeError(ctx, mod, "failed to read body from memory"))
			return
		}
		body = strings.NewReader(string(bodyBytes))
	}

	// TODO: Read and parse headers JSON (headersPtr, headersLen)
	// For now, ignore custom headers and use defaults
	_ = headersPtr
	_ = headersLen

	// Perform HTTP request
	start := time.Now()
	resp, err := performHTTPRequest(ctx, method, url, body)
	responseTime := time.Since(start).Milliseconds()

	if err != nil {
		stack[0] = uint64(writeError(ctx, mod, fmt.Sprintf("HTTP request failed: %v", err)))
		return
	}

	// Return success response
	stack[0] = uint64(writeHTTPSuccess(ctx, mod, resp, responseTime))
}

// performHTTPRequest executes the actual HTTP request
func performHTTPRequest(ctx context.Context, method, url string, body io.Reader) (*httpResponse, error) {
	// Create HTTP client
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		},
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set default headers
	req.Header.Set("User-Agent", "Reglet/1.0")

	// Perform request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Build response struct
	result := &httpResponse{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Headers:    make(map[string][]string),
		Body:       string(respBody),
	}

	// Copy headers
	for key, values := range resp.Header {
		result.Headers[key] = values
	}

	return result, nil
}

// httpResponse represents an HTTP response
type httpResponse struct {
	StatusCode int
	Status     string
	Headers    map[string][]string
	Body       string
}

// writeHTTPSuccess writes a successful HTTP response to WASM memory
func writeHTTPSuccess(ctx context.Context, mod api.Module, resp *httpResponse, responseTimeMs int64) uint32 {
	result := map[string]interface{}{
		"status":           true,
		"status_code":      resp.StatusCode,
		"status_text":      resp.Status,
		"headers":          resp.Headers,
		"body":             resp.Body,
		"response_time_ms": responseTimeMs,
	}
	return writeJSON(ctx, mod, result)
}
