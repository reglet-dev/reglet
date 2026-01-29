package hostfuncs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/reglet-dev/reglet-sdk/go/hostfuncs"
	"github.com/tetratelabs/wazero/api"
)

// SMTPConnect performs SMTP connection tests on behalf of the plugin.
// It receives a packed uint64 (ptr+len) pointing to a JSON-encoded SMTPRequestWire.
// It returns a packed uint64 (ptr+len) pointing to a JSON-encoded SMTPResponseWire.
func SMTPConnect(ctx context.Context, mod api.Module, stack []uint64, checker *CapabilityChecker) {
	// Stack contains a single uint64 which is packed ptr+len of the request.
	requestPacked := stack[0]
	ptr, length := unpackPtrLen(requestPacked)

	requestBytes, ok := mod.Memory().Read(ptr, length)
	if !ok {
		// Critical error, Host could not read Guest memory.
		errMsg := "hostfuncs: failed to read SMTP request from Guest memory"
		slog.ErrorContext(ctx, errMsg)
		stack[0] = hostWriteResponse(ctx, mod, SMTPResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "internal"},
		})
		return
	}

	var request SMTPRequestWire
	if err := json.Unmarshal(requestBytes, &request); err != nil {
		errMsg := fmt.Sprintf("hostfuncs: failed to unmarshal SMTP request: %v", err)
		slog.ErrorContext(ctx, errMsg)
		stack[0] = hostWriteResponse(ctx, mod, SMTPResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "internal"},
		})
		return
	}

	// 1. Check capability for outbound SMTP
	pluginName := mod.Name()
	if name, ok := PluginNameFromContext(ctx); ok {
		pluginName = name
	}

	port, _ := strconv.Atoi(request.Port) // Wire format uses string port
	if err := checker.CheckNetworkConnection(pluginName, request.Host, port); err != nil {
		errMsg := fmt.Sprintf("permission denied: %v", err)
		slog.WarnContext(ctx, errMsg, "host", request.Host, "port", request.Port)
		stack[0] = hostWriteResponse(ctx, mod, SMTPResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "capability"},
		})
		return
	}

	// 2. Build SDK request
	sdkReq := hostfuncs.SMTPConnectRequest{
		Host:        request.Host,
		Port:        port,
		UseTLS:      request.TLS,
		UseSTARTTLS: request.StartTLS,
		Timeout:     request.TimeoutMs,
	}

	// Determine if private IPs should be allowed via capability
	allowPrivate := checker.AllowsPrivateNetwork(pluginName)

	// Call SDK with SSRF protection
	sdkResp := hostfuncs.PerformSMTPConnect(ctx, sdkReq,
		hostfuncs.WithSMTPSSRFProtection(!allowPrivate),
	)

	// 3. Convert to wire format
	response := SMTPResponseWire{
		Connected:      sdkResp.Connected,
		Banner:         sdkResp.Banner,
		TLS:            sdkResp.TLSVersion != "",
		TLSVersion:     sdkResp.TLSVersion,
		ResponseTimeMs: sdkResp.LatencyMs,
	}

	response.TLSCipherSuite = sdkResp.TLSCipherSuite
	response.TLSServerName = sdkResp.TLSServerName

	if sdkResp.Error != nil {
		response.Error = &ErrorDetail{
			Message: sdkResp.Error.Message,
			Type:    sdkResp.Error.Code,
		}
	}

	stack[0] = hostWriteResponse(ctx, mod, response)
}
