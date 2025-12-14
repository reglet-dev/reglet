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
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/build"
)

// HTTPRequest performs an HTTP request on behalf of the plugin.
// It receives a packed uint64 (ptr+len) pointing to a JSON-encoded HTTPRequestWire.
// It returns a packed uint64 (ptr+len) pointing to a JSON-encoded HTTPResponseWire.
func HTTPRequest(ctx context.Context, mod api.Module, stack []uint64, checker *CapabilityChecker, version build.Info) {
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

	// SSRF protection: validate destination is not a private/reserved IP
	if err := ValidateDestination(ctx, parsedURL.Hostname(), pluginName, checker); err != nil {
		errMsg := fmt.Sprintf("SSRF protection: %v", err)
		slog.WarnContext(ctx, errMsg, "url", request.URL, "host", parsedURL.Hostname())
		stack[0] = hostWriteResponse(ctx, mod, HTTPResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "ssrf_protection"},
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

	// Set User-Agent first (Reglet/Version)
	// Plugins can override this via headers if needed, or we append if we use Add.
	// We use Set here to establish the default.
	userAgent := fmt.Sprintf("Reglet/%s (%s)", version.Version, version.Platform)
	req.Header.Set("User-Agent", userAgent)

	// Set headers from plugin
	for key, values := range request.Headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	// 4. Perform HTTP request with SSRF-safe client
	// SECURITY: Block redirects to prevent SSRF bypass via redirect chains
	// Redirects could bypass ValidateDestination by redirecting to private IPs
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Validate redirect target against SSRF protection
			if err := ValidateDestination(ctx, req.URL.Hostname(), pluginName, checker); err != nil {
				return fmt.Errorf("redirect blocked by SSRF protection: %w", err)
			}
			// Allow up to 10 redirects (http.DefaultClient default)
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		errMsg := fmt.Sprintf("HTTP request failed: %v", err)
		slog.ErrorContext(ctx, errMsg, "url", request.URL, "method", request.Method)
		stack[0] = hostWriteResponse(ctx, mod, HTTPResponseWire{
			Error: toErrorDetail(err),
		})
		return
	}
	defer func() {
		_ = resp.Body.Close() // Best-effort cleanup
	}()

	// 5. Read response body (with a limit) and detect truncation
	const maxBodySize = 10 * 1024 * 1024 // 10MB limit as per SDK design

	// Try to read maxBodySize + 1 byte to detect if body exceeds limit
	limitedReader := io.LimitReader(resp.Body, maxBodySize+1)
	respBodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		errMsg := fmt.Sprintf("failed to read response body: %v", err)
		slog.ErrorContext(ctx, errMsg, "url", request.URL)
		stack[0] = hostWriteResponse(ctx, mod, HTTPResponseWire{
			Error: toErrorDetail(err),
		})
		return
	}

	// Detect if body was truncated
	bodyTruncated := false
	if len(respBodyBytes) > maxBodySize {
		// Body exceeded limit - truncate and set flag
		respBodyBytes = respBodyBytes[:maxBodySize]
		bodyTruncated = true
		slog.WarnContext(ctx, "HTTP response body truncated",
			"url", request.URL,
			"max_size_mb", maxBodySize/(1024*1024),
			"truncated", true)
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
		StatusCode:    resp.StatusCode,
		Headers:       responseHeaders,
		Body:          encodedRespBody,
		BodyTruncated: bodyTruncated,
	})
}
