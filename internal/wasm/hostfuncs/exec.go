package hostfuncs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"github.com/tetratelabs/wazero/api"
)

// ExecCommand executes a command on the host
// signature: exec_command(reqPtr, reqLen) -> resPtr
func ExecCommand(ctx context.Context, mod api.Module, stack []uint64, checker *CapabilityChecker) {
	requestPacked := stack[0]
	ptr, length := unpackPtrLen(requestPacked)

	requestBytes, ok := mod.Memory().Read(ptr, length)
	if !ok {
		errMsg := "hostfuncs: failed to read Exec request from Guest memory"
		slog.ErrorContext(ctx, errMsg)
		stack[0] = hostWriteResponse(ctx, mod, ExecResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "internal"},
		})
		return
	}

	var request ExecRequestWire
	if err := json.Unmarshal(requestBytes, &request); err != nil {
		errMsg := fmt.Sprintf("hostfuncs: failed to unmarshal Exec request: %v", err)
		slog.ErrorContext(ctx, errMsg)
		stack[0] = hostWriteResponse(ctx, mod, ExecResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "internal"},
		})
		return
	}

	// Create context
	execCtx, cancel := createContextFromWire(ctx, request.Context)
	defer cancel()

	// Check capability
	pluginName := mod.Name()
	if name, ok := PluginNameFromContext(ctx); ok {
		pluginName = name
	}

	// Capability check: exec:<command>
	if err := checker.Check(pluginName, "exec", request.Command); err != nil {
		errMsg := fmt.Sprintf("permission denied: %v", err)
		slog.WarnContext(ctx, errMsg, "command", request.Command)
		stack[0] = hostWriteResponse(ctx, mod, ExecResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "capability"},
		})
		return
	}

	// Execute command
	cmd := exec.CommandContext(execCtx, request.Command, request.Args...)
	if request.Dir != "" {
		cmd.Dir = request.Dir
	}
	if len(request.Env) > 0 {
		cmd.Env = request.Env
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	var errorDetail *ErrorDetail

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// Other error (not found, timeout, etc.)
			errorDetail = toErrorDetail(err)
			// Map common exec errors
			if execCtx.Err() == context.DeadlineExceeded {
				errorDetail.Type = "timeout"
				errorDetail.Code = "ETIMEDOUT"
			} else {
				errorDetail.Type = "execution"
			}
		}
	}

	slog.DebugContext(ctx, "executed command",
		"command", request.Command,
		"args", request.Args,
		"exit_code", exitCode,
		"duration", duration,
		"error", err)

	// Write response
	stack[0] = hostWriteResponse(ctx, mod, ExecResponseWire{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Error:    errorDetail,
	})
}