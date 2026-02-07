package hostfuncs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"

	"github.com/reglet-dev/reglet-abi/hostfunc"
	hostlib "github.com/reglet-dev/reglet-hostlib"
	"github.com/reglet-dev/reglet/internal/infrastructure/build"
	"github.com/tetratelabs/wazero/api"
)

// HTTPRequest performs an HTTP request on behalf of the plugin.
func HTTPRequest(ctx context.Context, mod api.Module, stack []uint64, checker *CapabilityChecker, version build.Info) {
	request, err := readHTTPRequest(ctx, mod, stack[0])
	if err != nil {
		stack[0] = hostWriteResponse(ctx, mod, hostfunc.HTTPResponse{Error: err})
		return
	}

	// 1. Check capability
	pluginName := mod.Name()
	if name, ok := PluginNameFromContext(ctx); ok {
		pluginName = name
	}

	if err := checkHTTPCapability(ctx, checker, pluginName, request); err != nil {
		errMsg := fmt.Sprintf("permission denied: %v", err)
		slog.WarnContext(ctx, errMsg, "url", request.URL)
		stack[0] = hostWriteResponse(ctx, mod, hostfunc.HTTPResponse{
			Error: &hostfunc.ErrorDetail{Message: errMsg, Type: "capability"},
		})
		return
	}

	// 2. Build SDK request
	var body []byte
	if request.Body != "" {
		var decodeErr error
		body, decodeErr = base64.StdEncoding.DecodeString(request.Body)
		if decodeErr != nil {
			errMsg := fmt.Sprintf("failed to decode request body: %v", decodeErr)
			slog.ErrorContext(ctx, errMsg, "url", request.URL)
			stack[0] = hostWriteResponse(ctx, mod, hostfunc.HTTPResponse{
				Error: &hostfunc.ErrorDetail{Message: errMsg, Type: "config"},
			})
			return
		}
	}

	// Flatten headers for SDK (map[string][]string -> map[string]string)
	sdkHeaders := make(map[string]string)
	for k, v := range request.Headers {
		if len(v) > 0 {
			sdkHeaders[k] = strings.Join(v, ", ")
		}
	}

	// Add User-Agent if not present
	userAgent := fmt.Sprintf("Reglet/%s (%s)", version.Version, version.Platform)
	if _, ok := sdkHeaders["User-Agent"]; !ok {
		sdkHeaders["User-Agent"] = userAgent
	}

	sdkReq := hostlib.HTTPRequest{
		Method:  request.Method,
		URL:     request.URL,
		Headers: sdkHeaders,
		Body:    body,
		Timeout: int(request.Context.TimeoutMs),
	}

	// Determine if private network access is allowed
	allowPrivate := checker.AllowsPrivateNetwork(pluginName)

	// 3. Call SDK
	sdkResp := hostlib.PerformHTTPRequest(ctx, sdkReq,
		hostlib.WithHTTPSSRFProtection(!allowPrivate),
	)

	// 4. Convert to wire format
	var encodedRespBody string
	if len(sdkResp.Body) > 0 {
		encodedRespBody = base64.StdEncoding.EncodeToString(sdkResp.Body)
	}

	response := hostfunc.HTTPResponse{
		StatusCode:    sdkResp.StatusCode,
		Headers:       sdkResp.Headers,
		Body:          encodedRespBody,
		BodyTruncated: sdkResp.BodyTruncated,
		Proto:         sdkResp.Proto,
	}

	if sdkResp.Error != nil {
		response.Error = &hostfunc.ErrorDetail{
			Message: sdkResp.Error.Message,
			Type:    sdkResp.Error.Code,
		}
	}

	stack[0] = hostWriteResponse(ctx, mod, response)
}

// readHTTPRequest reads and unmarshals the HTTP request from guest memory.
func readHTTPRequest(ctx context.Context, mod api.Module, requestPacked uint64) (*hostfunc.HTTPRequest, *hostfunc.ErrorDetail) {
	ptr, length := unpackPtrLen(requestPacked)

	requestBytes, ok := mod.Memory().Read(ptr, length)
	if !ok {
		errMsg := "hostfuncs: failed to read HTTP request from Guest memory"
		slog.ErrorContext(ctx, errMsg)
		return nil, &hostfunc.ErrorDetail{Message: errMsg, Type: "internal"}
	}

	var request hostfunc.HTTPRequest
	if err := json.Unmarshal(requestBytes, &request); err != nil {
		errMsg := fmt.Sprintf("hostfuncs: failed to unmarshal HTTP request: %v", err)
		slog.ErrorContext(ctx, errMsg)
		return nil, &hostfunc.ErrorDetail{Message: errMsg, Type: "internal"}
	}

	return &request, nil
}

// checkHTTPCapability validates URL and checks network capability.
func checkHTTPCapability(ctx context.Context, checker *CapabilityChecker, pluginName string, request *hostfunc.HTTPRequest) error {
	// Simple wrapper around checker logic, matching previous behavior
	// Check specific host/port capability
	parsedURL, err := url.Parse(request.URL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	port := parsedURL.Port()
	if port == "" {
		if parsedURL.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	portInt, _ := strconv.Atoi(port)
	return checker.CheckNetworkConnection(pluginName, parsedURL.Hostname(), portInt)
}
