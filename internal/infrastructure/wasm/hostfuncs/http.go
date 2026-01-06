package hostfuncs

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url" // Import url for URL parsing
	"time"

	"github.com/tetratelabs/wazero/api"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/build"
)

// dnsPinningTransport is a custom http.RoundTripper that prevents DNS rebinding attacks
// by resolving DNS once, validating the IP, and connecting to that specific IP.
type dnsPinningTransport struct {
	base       *http.Transport
	ctx        context.Context
	pluginName string
	checker    *CapabilityChecker
}

// RoundTrip implements http.RoundTripper with DNS pinning and SSRF protection.
func (t *dnsPinningTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	hostname := req.URL.Hostname()

	// Resolve and validate hostname to IP (prevents DNS rebinding)
	validatedIP, err := resolveAndValidate(t.ctx, hostname, t.pluginName, t.checker)
	if err != nil {
		return nil, fmt.Errorf("SSRF protection: %w", err)
	}

	// Determine port
	port := req.URL.Port()
	if port == "" {
		if req.URL.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	// Create a new transport with DNS pinned to validated IP
	// Clone base transport settings to preserve configuration
	pinnedTransport := t.base.Clone()
	pinnedTransport.DialContext = func(dialCtx context.Context, network, _ string) (net.Conn, error) { // addr bypassed with validatedIP
		// Ignore addr parameter and use validated IP
		// This ensures connection goes to IP we validated, not a potentially rebinded DNS result
		targetAddr := net.JoinHostPort(validatedIP, port)
		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}
		return dialer.DialContext(dialCtx, network, targetAddr)
	}

	// For HTTPS, preserve hostname for SNI and certificate validation
	if req.URL.Scheme == "https" {
		if pinnedTransport.TLSClientConfig == nil {
			pinnedTransport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		}
		pinnedTransport.TLSClientConfig.ServerName = hostname
	}

	return pinnedTransport.RoundTrip(req)
}

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

	// NOTE: SSRF protection with DNS pinning happens in dnsPinningTransport.RoundTrip()
	// This validates and pins DNS for both initial request and redirects

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

	// 4. Perform HTTP request with SSRF-safe client using DNS pinning
	// SECURITY: Create custom RoundTripper that validates and pins DNS for each request
	// This prevents DNS rebinding attacks on both initial request and redirects
	baseTransport := &http.Transport{
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// Custom RoundTripper that validates and pins DNS before each request
	dnsPinnedTransport := &dnsPinningTransport{
		base:       baseTransport,
		ctx:        ctx,
		pluginName: pluginName,
		checker:    checker,
	}

	client := &http.Client{
		Transport: dnsPinnedTransport,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			// Allow up to 10 redirects (http.DefaultClient default)
			// DNS validation happens in RoundTrip for each request
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
