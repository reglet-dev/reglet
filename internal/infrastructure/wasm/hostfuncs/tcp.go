package hostfuncs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/reglet-dev/reglet-sdk/go/hostfuncs"
	"github.com/tetratelabs/wazero/api"
)

// TCPConnect performs TCP connection tests on behalf of the plugin.
// It receives a packed uint64 (ptr+len) pointing to a JSON-encoded TCPRequestWire.
// It returns a packed uint64 (ptr+len) pointing to a JSON-encoded TCPResponseWire.
func TCPConnect(ctx context.Context, mod api.Module, stack []uint64, checker *CapabilityChecker) {
	// Stack contains a single uint64 which is packed ptr+len of the request.
	requestPacked := stack[0]
	ptr, length := unpackPtrLen(requestPacked)

	requestBytes, ok := mod.Memory().Read(ptr, length)
	if !ok {
		// Critical error, Host could not read Guest memory.
		errMsg := "hostfuncs: failed to read TCP request from Guest memory"
		slog.ErrorContext(ctx, errMsg)
		stack[0] = hostWriteResponse(ctx, mod, TCPResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "internal"},
		})
		return
	}

	var request TCPRequestWire
	if err := json.Unmarshal(requestBytes, &request); err != nil {
		errMsg := fmt.Sprintf("hostfuncs: failed to unmarshal TCP request: %v", err)
		slog.ErrorContext(ctx, errMsg)
		stack[0] = hostWriteResponse(ctx, mod, TCPResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "internal"},
		})
		return
	}

	// 1. Check capability for outbound TCP
	pluginName := mod.Name()
	if name, ok := PluginNameFromContext(ctx); ok {
		pluginName = name
	}

	// Check specific connection capability using the new unified method.
	// This creates a specific request (Host+Port) and checks if ANY rule allows it.
	port, _ := strconv.Atoi(request.Port) // Error check handled by capability check or SDK later
	err := checker.CheckNetworkConnection(pluginName, request.Host, port)
	if err != nil {
		errMsg := fmt.Sprintf("permission denied: %v", err)
		slog.WarnContext(ctx, errMsg, "host", request.Host, "port", request.Port)
		stack[0] = hostWriteResponse(ctx, mod, TCPResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "capability"},
		})
		return
	}

	// 2. Prepare SDK request
	sdkReq := hostfuncs.TCPConnectRequest{
		Host:    request.Host,
		Port:    port,
		Timeout: request.TimeoutMs,
		UseTLS:  request.TLS,
	}

	// Determine if private IPs should be allowed via capability
	allowPrivate := checker.AllowsPrivateNetwork(pluginName)

	// Call SDK with SSRF protection
	sdkResp := hostfuncs.PerformTCPConnect(ctx, sdkReq,
		hostfuncs.WithTCPSSRFProtection(!allowPrivate),
	)

	// 3. Convert to wire format
	response := TCPResponseWire{
		Connected:      sdkResp.Connected,
		LocalAddr:      "",
		RemoteAddr:     sdkResp.RemoteAddr,
		ResponseTimeMs: sdkResp.LatencyMs,
		TLS:            sdkResp.TLSVersion != "", // If TLS version is set, TLS was used
		TLSVersion:     sdkResp.TLSVersion,
		TLSCipherSuite: sdkResp.TLSCipherSuite,
		TLSServerName:  sdkResp.TLSServerName,
		TLSCertSubject: sdkResp.TLSCertSubject,
		TLSCertIssuer:  sdkResp.TLSCertIssuer,
	}

	if sdkResp.TLSCertExpiry != "" {
		if t, err := time.Parse(time.RFC3339, sdkResp.TLSCertExpiry); err == nil {
			response.TLSCertNotAfter = &t
		}
	}

	if sdkResp.Error != nil {
		response.Error = &ErrorDetail{
			Message: sdkResp.Error.Message,
			Type:    sdkResp.Error.Code,
		}
	}

	stack[0] = hostWriteResponse(ctx, mod, response)
}
