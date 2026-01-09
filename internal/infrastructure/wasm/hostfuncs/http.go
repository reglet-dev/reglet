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
	"net/url"
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

	port := getPort(req.URL)
	pinnedTransport := t.createPinnedTransport(validatedIP, port, hostname, req.URL.Scheme)

	return pinnedTransport.RoundTrip(req)
}

// getPort returns the port for a URL, defaulting based on scheme.
func getPort(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}
	if u.Scheme == "https" {
		return "443"
	}
	return "80"
}

// createPinnedTransport creates a transport that connects to the validated IP.
func (t *dnsPinningTransport) createPinnedTransport(validatedIP, port, hostname, scheme string) *http.Transport {
	pinnedTransport := t.base.Clone()
	pinnedTransport.DialContext = func(dialCtx context.Context, network, _ string) (net.Conn, error) {
		targetAddr := net.JoinHostPort(validatedIP, port)
		dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
		return dialer.DialContext(dialCtx, network, targetAddr)
	}

	if scheme == "https" {
		if pinnedTransport.TLSClientConfig == nil {
			pinnedTransport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		}
		pinnedTransport.TLSClientConfig.ServerName = hostname
	}

	return pinnedTransport
}

// HTTPRequest performs an HTTP request on behalf of the plugin.
func HTTPRequest(ctx context.Context, mod api.Module, stack []uint64, checker *CapabilityChecker, version build.Info) {
	request, err := readHTTPRequest(ctx, mod, stack[0])
	if err != nil {
		stack[0] = hostWriteResponse(ctx, mod, HTTPResponseWire{Error: err})
		return
	}

	httpCtx, cancel := createContextFromWire(ctx, request.Context)
	defer cancel()

	pluginName := getPluginName(ctx, mod)

	if err := checkHTTPCapability(ctx, checker, pluginName, request); err != nil {
		stack[0] = hostWriteResponse(ctx, mod, HTTPResponseWire{Error: err})
		return
	}

	req, err := buildHTTPRequest(ctx, httpCtx, request, version)
	if err != nil {
		stack[0] = hostWriteResponse(ctx, mod, HTTPResponseWire{Error: err})
		return
	}

	response := executeHTTPRequest(ctx, req, pluginName, checker, request.URL)
	stack[0] = hostWriteResponse(ctx, mod, response)
}

// readHTTPRequest reads and unmarshals the HTTP request from guest memory.
func readHTTPRequest(ctx context.Context, mod api.Module, requestPacked uint64) (*HTTPRequestWire, *ErrorDetail) {
	ptr, length := unpackPtrLen(requestPacked)

	requestBytes, ok := mod.Memory().Read(ptr, length)
	if !ok {
		errMsg := "hostfuncs: failed to read HTTP request from Guest memory"
		slog.ErrorContext(ctx, errMsg)
		return nil, &ErrorDetail{Message: errMsg, Type: "internal"}
	}

	var request HTTPRequestWire
	if err := json.Unmarshal(requestBytes, &request); err != nil {
		errMsg := fmt.Sprintf("hostfuncs: failed to unmarshal HTTP request: %v", err)
		slog.ErrorContext(ctx, errMsg)
		return nil, &ErrorDetail{Message: errMsg, Type: "internal"}
	}

	return &request, nil
}

// checkHTTPCapability validates URL and checks network capability.
func checkHTTPCapability(ctx context.Context, checker *CapabilityChecker, pluginName string, request *HTTPRequestWire) *ErrorDetail {
	parsedURL, err := url.Parse(request.URL)
	if err != nil {
		errMsg := fmt.Sprintf("invalid URL: %v", err)
		slog.WarnContext(ctx, errMsg, "url", request.URL)
		return &ErrorDetail{Message: errMsg, Type: "config"}
	}

	port := getPort(parsedURL)
	capabilityPattern := fmt.Sprintf("outbound:%s", port)

	if err := checker.Check(pluginName, "network", capabilityPattern); err != nil {
		errMsg := fmt.Sprintf("permission denied for %s %s: %v", request.Method, request.URL, err)
		slog.WarnContext(ctx, errMsg, "url", request.URL, "method", request.Method)
		return &ErrorDetail{Message: errMsg, Type: "capability"}
	}

	return nil
}

// buildHTTPRequest creates the native http.Request from wire format.
func buildHTTPRequest(ctx context.Context, httpCtx context.Context, request *HTTPRequestWire, version build.Info) (*http.Request, *ErrorDetail) {
	var reqBody io.Reader
	if request.Body != "" {
		decodedBody, err := base64.StdEncoding.DecodeString(request.Body)
		if err != nil {
			errMsg := fmt.Sprintf("failed to decode request body: %v", err)
			slog.ErrorContext(ctx, errMsg, "url", request.URL)
			return nil, &ErrorDetail{Message: errMsg, Type: "config"}
		}
		reqBody = bytes.NewReader(decodedBody)
	}

	req, err := http.NewRequestWithContext(httpCtx, request.Method, request.URL, reqBody)
	if err != nil {
		errMsg := fmt.Sprintf("failed to create HTTP request: %v", err)
		slog.ErrorContext(ctx, errMsg, "url", request.URL, "method", request.Method)
		return nil, &ErrorDetail{Message: errMsg, Type: "internal"}
	}

	userAgent := fmt.Sprintf("Reglet/%s (%s)", version.Version, version.Platform)
	req.Header.Set("User-Agent", userAgent)

	for key, values := range request.Headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	return req, nil
}

// executeHTTPRequest performs the HTTP request and returns the response.
func executeHTTPRequest(ctx context.Context, req *http.Request, pluginName string, checker *CapabilityChecker, requestURL string) HTTPResponseWire {
	baseTransport := &http.Transport{
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := &http.Client{
		Transport: &dnsPinningTransport{
			base:       baseTransport,
			ctx:        ctx,
			pluginName: pluginName,
			checker:    checker,
		},
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		errMsg := fmt.Sprintf("HTTP request failed: %v", err)
		slog.ErrorContext(ctx, errMsg, "url", requestURL, "method", req.Method)
		return HTTPResponseWire{Error: toErrorDetail(err)}
	}
	defer func() { _ = resp.Body.Close() }()

	return readHTTPResponse(ctx, resp, requestURL)
}

// readHTTPResponse reads and encodes the HTTP response.
func readHTTPResponse(ctx context.Context, resp *http.Response, requestURL string) HTTPResponseWire {
	const maxBodySize = 10 * 1024 * 1024 // 10MB limit

	limitedReader := io.LimitReader(resp.Body, maxBodySize+1)
	respBodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		errMsg := fmt.Sprintf("failed to read response body: %v", err)
		slog.ErrorContext(ctx, errMsg, "url", requestURL)
		return HTTPResponseWire{Error: toErrorDetail(err)}
	}

	bodyTruncated := false
	if len(respBodyBytes) > maxBodySize {
		respBodyBytes = respBodyBytes[:maxBodySize]
		bodyTruncated = true
		slog.WarnContext(ctx, "HTTP response body truncated",
			"url", requestURL,
			"max_size_mb", maxBodySize/(1024*1024),
			"truncated", true)
	}

	var encodedRespBody string
	if len(respBodyBytes) > 0 {
		encodedRespBody = base64.StdEncoding.EncodeToString(respBodyBytes)
	}

	responseHeaders := make(map[string][]string)
	for key, values := range resp.Header {
		responseHeaders[key] = values
	}

	return HTTPResponseWire{
		StatusCode:    resp.StatusCode,
		Headers:       responseHeaders,
		Body:          encodedRespBody,
		BodyTruncated: bodyTruncated,
	}
}
