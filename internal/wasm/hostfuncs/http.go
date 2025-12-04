package hostfuncs

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url" // Import url for URL parsing

	"github.com/tetratelabs/wazero/api"
)

// HTTPRequest performs an HTTP request on behalf of the plugin.
// It receives a packed uint64 (ptr+len) pointing to a JSON-encoded HTTPRequestWire.
// It returns a packed uint64 (ptr+len) pointing to a JSON-encoded HTTPResponseWire.
func HTTPRequest(ctx context.Context, mod api.Module, stack []uint64, checker *CapabilityChecker) {
	// Stack contains a single uint64 which is packed ptr+len of the request.
	requestPacked := stack[0]
	ptr, length := unpackPtrLen(requestPacked)

	requestBytes, ok := mod.Memory().Read(ptr, length)
	if !ok {
		// Critical error, Host could not read Guest memory.
		errMsg := "hostfuncs: failed to read HTTP request from Guest memory"
		slog.ErrorContext(ctx, errMsg)
		stack[0] = hostWriteResponse(ctx, mod, HTTPResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "internal"},
		})
		return
	}

	var request HTTPRequestWire
	if err := json.Unmarshal(requestBytes, &request); err != nil {
		errMsg := fmt.Sprintf("hostfuncs: failed to unmarshal HTTP request: %v", err)
		slog.ErrorContext(ctx, errMsg)
		stack[0] = hostWriteResponse(ctx, mod, HTTPResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "internal"},
		})
		return
	}

	// Create a new context from the wire format, with parent ctx for cancellation.
	httpCtx, cancel := createContextFromWire(ctx, request.Context)
	defer cancel() // Ensure context resources are released.

	// 1. Check capability
	parsedURL, err := url.Parse(request.URL)
	if err != nil {
		errMsg := fmt.Sprintf("invalid URL: %v", err)
		slog.WarnContext(ctx, errMsg, "url", request.URL)
		stack[0] = hostWriteResponse(ctx, mod, HTTPResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "config"},
		})
		return
	}

	port := parsedURL.Port()
	if port == "" {
		if parsedURL.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	capabilityPattern := fmt.Sprintf("outbound:%s", port)

	pluginName := mod.Name()
	if name, ok := PluginNameFromContext(ctx); ok {
		pluginName = name
	}

	if err := checker.Check(pluginName, "network", capabilityPattern); err != nil {
		errMsg := fmt.Sprintf("permission denied for %s %s: %v", request.Method, request.URL, err)
		slog.WarnContext(ctx, errMsg, "url", request.URL, "method", request.Method)
		stack[0] = hostWriteResponse(ctx, mod, HTTPResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "capability"},
		})
		return
	}

	// 2. Prepare HTTP request body
	var reqBody io.Reader
	if request.Body != "" {
		// Assume base64 for now, as per SDK design
		decodedBody, err := base64.StdEncoding.DecodeString(request.Body)
		if err != nil {
			errMsg := fmt.Sprintf("failed to decode request body: %v", err)
			slog.ErrorContext(ctx, errMsg, "url", request.URL)
			stack[0] = hostWriteResponse(ctx, mod, HTTPResponseWire{
				Error: &ErrorDetail{Message: errMsg, Type: "config"},
			})
			return
		}
		reqBody = bytes.NewReader(decodedBody)
	}

	// 3. Create native http.Request
	req, err := http.NewRequestWithContext(httpCtx, request.Method, request.URL, reqBody)
	if err != nil {
		errMsg := fmt.Sprintf("failed to create HTTP request: %v", err)
		slog.ErrorContext(ctx, errMsg, "url", request.URL, "method", request.Method)
		stack[0] = hostWriteResponse(ctx, mod, HTTPResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "internal"},
		})
		return
	}

	// Set headers
	for key, values := range request.Headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// 4. Perform HTTP request
	client := http.DefaultClient // Use default client for now (could be configurable later)
	resp, err := client.Do(req)
	if err != nil {
		errMsg := fmt.Sprintf("HTTP request failed: %v", err)
		slog.ErrorContext(ctx, errMsg, "url", request.URL, "method", request.Method)
		stack[0] = hostWriteResponse(ctx, mod, HTTPResponseWire{
			Error: toErrorDetail(err),
		})
		return
	}
	defer resp.Body.Close()

	// 5. Read response body (with a limit)
	const maxBodySize = 10 * 1024 * 1024 // 10MB limit as per SDK design
	respBodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		errMsg := fmt.Sprintf("failed to read response body: %v", err)
		slog.ErrorContext(ctx, errMsg, "url", request.URL)
		stack[0] = hostWriteResponse(ctx, mod, HTTPResponseWire{
			Error: toErrorDetail(err),
		})
		return
	}
	var encodedRespBody string
	if len(respBodyBytes) > 0 {
		encodedRespBody = base64.StdEncoding.EncodeToString(respBodyBytes)
	}

	// 6. Prepare HTTPResponseWire
	responseHeaders := make(map[string][]string)
	for key, values := range resp.Header {
		responseHeaders[key] = values
	}

	stack[0] = hostWriteResponse(ctx, mod, HTTPResponseWire{
		StatusCode: resp.StatusCode,
		Headers:    responseHeaders,
		Body:       encodedRespBody,
	})
}