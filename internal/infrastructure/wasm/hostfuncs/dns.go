package hostfuncs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/reglet-dev/reglet-sdk/go/hostfuncs"
	"github.com/tetratelabs/wazero/api"
)

// DNSLookup performs DNS resolution on behalf of the plugin.
// It receives a packed uint64 (ptr+len) pointing to a JSON-encoded DNSRequestWire.
// It returns a packed uint64 (ptr+len) pointing to a JSON-encoded DNSResponseWire.
//
// This handler:
// 1. Reads request from guest memory
// 2. Checks capability (network:outbound:53)
// 3. Delegates to SDK's PerformDNSLookup
// 4. Writes response to guest memory
func DNSLookup(ctx context.Context, mod api.Module, stack []uint64, checker *CapabilityChecker) {
	requestPacked := stack[0]
	ptr, length := unpackPtrLen(requestPacked)

	requestBytes, ok := mod.Memory().Read(ptr, length)
	if !ok {
		errMsg := "hostfuncs: failed to read DNS request from Guest memory"
		slog.ErrorContext(ctx, errMsg)
		stack[0] = hostWriteResponse(ctx, mod, DNSResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "internal"},
		})
		return
	}

	var request DNSRequestWire
	if err := json.Unmarshal(requestBytes, &request); err != nil {
		errMsg := fmt.Sprintf("hostfuncs: failed to unmarshal DNS request: %v", err)
		slog.ErrorContext(ctx, errMsg)
		stack[0] = hostWriteResponse(ctx, mod, DNSResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "internal"},
		})
		return
	}

	// Check capability
	pluginName := mod.Name()
	if name, ok := PluginNameFromContext(ctx); ok {
		pluginName = name
	}

	if err := checker.Check(pluginName, "network", "outbound:53"); err != nil {
		errMsg := fmt.Sprintf("permission denied: %v", err)
		slog.WarnContext(ctx, errMsg, "hostname", request.Hostname)
		stack[0] = hostWriteResponse(ctx, mod, DNSResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "capability"},
		})
		return
	}

	// Create SDK request and delegate to SDK's PerformDNSLookup
	sdkReq := hostfuncs.DNSLookupRequest{
		Hostname:   request.Hostname,
		RecordType: request.Type,
		Nameserver: request.Nameserver,
	}

	// Apply timeout from wire context if present
	if request.Context.TimeoutMs > 0 {
		sdkReq.Timeout = int(request.Context.TimeoutMs)
	}

	sdkResp := hostfuncs.PerformDNSLookup(ctx, sdkReq)

	// Convert SDK response to wire format
	response := DNSResponseWire{
		Records: sdkResp.Records,
	}

	if sdkResp.Error != nil {
		response.Error = &ErrorDetail{
			Message: sdkResp.Error.Message,
			Type:    "network",
		}
	}

	// Convert MX records if present
	for _, mx := range sdkResp.MXRecords {
		response.MXRecords = append(response.MXRecords, MXRecordWire{
			Host: mx.Host,
			Pref: mx.Pref,
		})
	}

	stack[0] = hostWriteResponse(ctx, mod, response)
}
