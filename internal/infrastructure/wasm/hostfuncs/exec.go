package hostfuncs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	sdkEntities "github.com/reglet-dev/reglet-sdk/domain/entities"
	"github.com/reglet-dev/reglet-hostlib"
	"github.com/reglet-dev/reglet/internal/domain/constants"
	"github.com/tetratelabs/wazero/api"
)

// ExecCommand executes a command on the host.
// It receives a packed uint64 (ptr+len) pointing to a JSON-encoded ExecRequestWire.
// It returns a packed uint64 (ptr+len) pointing to a JSON-encoded ExecResponseWire.
//
// This handler:
// 1. Reads request from guest memory
// 2. Checks capability (exec:<command>) with shell/interpreter detection
// 3. Delegates to SDK's PerformSecureExecCommand for actual execution
// 4. Writes response to guest memory
func ExecCommand(ctx context.Context, mod api.Module, stack []uint64, checker *CapabilityChecker) {
	request, err := readExecRequest(ctx, mod, stack[0])
	if err != nil {
		stack[0] = hostWriteResponse(ctx, mod, ExecResponseWire{
			Error: &ErrorDetail{Message: err.Error(), Type: "internal"},
		})
		return
	}

	// Create context
	execCtx, cancel := createContextFromWire(ctx, request.Context)
	defer cancel()

	// Check capability
	pluginName := getPluginName(ctx, mod)

	if err := checkExecCapability(ctx, checker, pluginName, request, stack, mod); err != nil {
		return // Response already written
	}

	// Create SDK request
	sdkReq := hostlib.ExecCommandRequest{
		Command: request.Command,
		Args:    request.Args,
		Dir:     request.Dir,
		Env:     request.Env,
	}

	// Apply timeout from wire context if present
	if request.Context.TimeoutMs > 0 {
		sdkReq.Timeout = int(request.Context.TimeoutMs)
	}

	// Delegate to SDK's secure exec with capability getter
	capGetter := checker.ToCapabilityGetter(pluginName)
	sdkResp := hostlib.PerformSecureExecCommand(execCtx, sdkReq, pluginName, capGetter)

	// Convert SDK response to wire format
	response := ExecResponseWire{
		Stdout:     sdkResp.Stdout,
		Stderr:     sdkResp.Stderr,
		ExitCode:   sdkResp.ExitCode,
		DurationMs: sdkResp.DurationMs,
		IsTimeout:  sdkResp.IsTimeout,
	}

	if sdkResp.Error != nil {
		response.Error = &ErrorDetail{
			Message: sdkResp.Error.Message,
			Type:    "execution",
			Code:    sdkResp.Error.Code,
		}
	}

	// Log truncation if it occurred
	if sdkResp.StdoutTruncated || sdkResp.StderrTruncated {
		slog.WarnContext(ctx, "command output truncated",
			"command", request.Command,
			"stdout_truncated", sdkResp.StdoutTruncated,
			"stderr_truncated", sdkResp.StderrTruncated)
	}

	slog.DebugContext(ctx, "executed command",
		"command", request.Command,
		"args", request.Args,
		"exit_code", response.ExitCode,
		"duration_ms", response.DurationMs,
		"error", sdkResp.Error)

	stack[0] = hostWriteResponse(ctx, mod, response)
}

// MaxRequestSize limits the size of incoming requests from guest memory (1MB).
// This prevents malicious WASM modules from triggering OOM by claiming huge request sizes.
// This is a NON-CONFIGURABLE security limit (same as constants.MaxRequestSize).
const MaxRequestSize = constants.MaxRequestSize

// readExecRequest reads and unmarshals the exec request from guest memory.
func readExecRequest(ctx context.Context, mod api.Module, requestPacked uint64) (*ExecRequestWire, error) {
	ptr, length := unpackPtrLen(requestPacked)

	// SECURITY: Enforce request size limit before allocating memory
	if length > MaxRequestSize {
		errMsg := fmt.Sprintf("hostfuncs: request size %d exceeds maximum allowed %d bytes", length, MaxRequestSize)
		slog.ErrorContext(ctx, errMsg)
		return nil, errors.New(errMsg)
	}

	requestBytes, ok := mod.Memory().Read(ptr, length)
	if !ok {
		errMsg := "hostfuncs: failed to read Exec request from Guest memory"
		slog.ErrorContext(ctx, errMsg)
		return nil, errors.New(errMsg)
	}

	var request ExecRequestWire
	if err := json.Unmarshal(requestBytes, &request); err != nil {
		errMsg := fmt.Sprintf("hostfuncs: failed to unmarshal Exec request: %v", err)
		slog.ErrorContext(ctx, errMsg)
		return nil, errors.New(errMsg)
	}

	return &request, nil
}

// getPluginName extracts the plugin name from context or module.
func getPluginName(ctx context.Context, mod api.Module) string {
	if name, ok := PluginNameFromContext(ctx); ok {
		return name
	}
	return mod.Name()
}

// checkExecCapability verifies the plugin has permission to execute the command.
// Uses SDK's DetectExecutionType for shell/interpreter detection.
// Returns nil on success, writes error response and returns error on failure.
func checkExecCapability(ctx context.Context, checker *CapabilityChecker, pluginName string, request *ExecRequestWire, stack []uint64, mod api.Module) error {
	// Use SDK's execution type detection
	execType := hostlib.GetExecutionTypeDescription(request.Command, request.Args)

	if hostlib.IsDangerousExecution(request.Command, request.Args) {
		return checkDangerousExec(ctx, checker, pluginName, request, execType, stack, mod)
	}

	// Direct command execution (safe mode)
	if err := checker.CheckExec(pluginName, sdkEntities.ExecCapabilityRequest{Command: request.Command}); err != nil {
		errMsg := fmt.Sprintf("permission denied: %v", err)
		slog.WarnContext(ctx, errMsg, "command", request.Command)
		stack[0] = hostWriteResponse(ctx, mod, ExecResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "capability"},
		})
		return errors.New(errMsg)
	}

	return nil
}

// checkDangerousExec handles capability check for dangerous execution modes.
func checkDangerousExec(ctx context.Context, checker *CapabilityChecker, pluginName string, request *ExecRequestWire, execType string, stack []uint64, mod api.Module) error {
	if err := checker.CheckExec(pluginName, sdkEntities.ExecCapabilityRequest{Command: request.Command}); err != nil {
		errMsg := fmt.Sprintf(
			"%s requires 'exec:%s' capability (prevents arbitrary code execution)",
			execType, request.Command)
		slog.WarnContext(ctx, errMsg,
			"command", request.Command,
			"args", request.Args,
			"type", execType,
			"plugin", pluginName)
		stack[0] = hostWriteResponse(ctx, mod, ExecResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "capability"},
		})
		return errors.New(errMsg)
	}

	slog.InfoContext(ctx, "dangerous execution granted",
		"command", request.Command,
		"args", request.Args,
		"type", execType,
		"plugin", pluginName)

	return nil
}
