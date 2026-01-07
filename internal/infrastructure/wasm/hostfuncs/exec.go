package hostfuncs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/tetratelabs/wazero/api"
)

// ExecCommand executes a command on the host
// signature: exec_command(reqPtr, reqLen) -> resPtr
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

	// Execute and write response
	response := executeCommand(ctx, execCtx, request)
	stack[0] = hostWriteResponse(ctx, mod, response)
}

// readExecRequest reads and unmarshals the exec request from guest memory.
func readExecRequest(ctx context.Context, mod api.Module, requestPacked uint64) (*ExecRequestWire, error) {
	ptr, length := unpackPtrLen(requestPacked)

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

// executionType represents the type of command execution.
type executionType string

const (
	execTypeSafe        executionType = "safe"
	execTypeShell       executionType = "shell"
	execTypeInterpreter executionType = "interpreter code execution"
	execTypeSuspicious  executionType = "suspicious execution"
)

// detectExecutionType determines if the command is dangerous and what type.
func detectExecutionType(command string, args []string) executionType {
	if isShellExecution(command) && len(args) > 0 {
		return execTypeShell
	}
	if hasCodeExecutionFlags(command, args) {
		return execTypeInterpreter
	}
	if hasSuspiciousFlags(args) {
		return execTypeSuspicious
	}
	return execTypeSafe
}

// checkExecCapability verifies the plugin has permission to execute the command.
// Returns nil on success, writes error response and returns error on failure.
func checkExecCapability(ctx context.Context, checker *CapabilityChecker, pluginName string, request *ExecRequestWire, stack []uint64, mod api.Module) error {
	execType := detectExecutionType(request.Command, request.Args)

	if execType != execTypeSafe {
		return checkDangerousExec(ctx, checker, pluginName, request, execType, stack, mod)
	}

	// Direct command execution (safe mode)
	if err := checker.Check(pluginName, "exec", request.Command); err != nil {
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
func checkDangerousExec(ctx context.Context, checker *CapabilityChecker, pluginName string, request *ExecRequestWire, execType executionType, stack []uint64, mod api.Module) error {
	if err := checker.Check(pluginName, "exec", request.Command); err != nil {
		errMsg := fmt.Sprintf(
			"%s requires 'exec:%s' capability (prevents arbitrary code execution)",
			execType, request.Command)
		slog.WarnContext(ctx, errMsg,
			"command", request.Command,
			"args", request.Args,
			"type", string(execType),
			"plugin", pluginName)
		stack[0] = hostWriteResponse(ctx, mod, ExecResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "capability"},
		})
		return errors.New(errMsg)
	}

	slog.InfoContext(ctx, "dangerous execution granted",
		"command", request.Command,
		"args", request.Args,
		"type", string(execType),
		"plugin", pluginName)

	return nil
}

// executeCommand runs the command and returns the response.
func executeCommand(ctx, execCtx context.Context, request *ExecRequestWire) ExecResponseWire {
	//nolint:gosec // G204: capability system validates commands; no shell interpretation
	cmd := exec.CommandContext(execCtx, request.Command, request.Args...)

	if request.Dir != "" {
		cmd.Dir = request.Dir
	}

	// SECURITY: Always set cmd.Env explicitly to prevent host environment leakage
	if len(request.Env) > 0 {
		cmd.Env = request.Env
	} else {
		cmd.Env = []string{}
	}

	// 10MB limit for stdout/stderr to prevent OOM DoS
	const MaxOutputSize = 10 * 1024 * 1024
	stdout := NewBoundedBuffer(MaxOutputSize)
	stderr := NewBoundedBuffer(MaxOutputSize)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	response := buildExecResponse(ctx, execCtx, err, stdout, stderr, duration, request.Command, request.Args)

	if stdout.Truncated || stderr.Truncated {
		slog.WarnContext(ctx, "command output truncated",
			"command", request.Command,
			"stdout_truncated", stdout.Truncated,
			"stderr_truncated", stderr.Truncated)
	}

	slog.DebugContext(ctx, "executed command",
		"command", request.Command,
		"args", request.Args,
		"exit_code", response.ExitCode,
		"duration", duration,
		"error", err)

	return response
}

// buildExecResponse constructs the response from command execution results.
func buildExecResponse(ctx, execCtx context.Context, err error, stdout, stderr *BoundedBuffer, duration time.Duration, command string, args []string) ExecResponseWire {
	_ = ctx
	response := ExecResponseWire{
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		ExitCode:   0,
		DurationMs: duration.Milliseconds(),
	}

	if err == nil {
		return response
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		response.ExitCode = exitErr.ExitCode()
		return response
	}

	// Other error (not found, timeout, etc.)
	response.Error = toErrorDetail(err)
	if execCtx.Err() == context.DeadlineExceeded {
		response.Error.Type = "timeout"
		response.Error.Code = "ETIMEDOUT"
		response.IsTimeout = true
	} else {
		response.Error.Type = "execution"
	}

	return response
}

// BoundedBuffer is a bytes.Buffer wrapper that limits the size of written data.
type BoundedBuffer struct {
	buffer    bytes.Buffer
	limit     int
	Truncated bool
}

// NewBoundedBuffer creates a new BoundedBuffer with the specified limit.
func NewBoundedBuffer(limit int) *BoundedBuffer {
	return &BoundedBuffer{
		limit: limit,
	}
}

// Write implements io.Writer.
func (b *BoundedBuffer) Write(p []byte) (n int, err error) {
	if b.buffer.Len() >= b.limit {
		b.Truncated = true
		return len(p), nil // Pretend we wrote it all to satisfy io.Writer contract
	}

	remaining := b.limit - b.buffer.Len()
	if len(p) > remaining {
		b.Truncated = true
		n, err = b.buffer.Write(p[:remaining])
		if err != nil {
			return n, err
		}
		return len(p), nil // Return len(p) to avoid short write error
	}

	return b.buffer.Write(p)
}

// String returns the buffer contents as a string.
func (b *BoundedBuffer) String() string {
	return b.buffer.String()
}

// isShellExecution detects if a command is a shell invocation.
// Common shells: sh, bash, dash, zsh, ksh, csh, tcsh, fish
func isShellExecution(command string) bool {
	base := getBasename(command)
	shells := []string{"sh", "bash", "dash", "zsh", "ksh", "csh", "tcsh", "fish"}
	for _, shell := range shells {
		if base == shell {
			return true
		}
	}
	return false
}

// getBasename extracts the binary name from a path.
func getBasename(command string) string {
	if idx := strings.LastIndex(command, "/"); idx >= 0 {
		return command[idx+1:]
	}
	return command
}

// isKnownInterpreter detects if a command is a known scripting interpreter.
func isKnownInterpreter(command string) bool {
	base := getBasename(command)
	interpreters := []string{
		"python", "python2", "python3",
		"python2.7", "python3.6", "python3.7", "python3.8", "python3.9", "python3.10", "python3.11", "python3.12",
		"perl", "perl5",
		"ruby", "irb",
		"node", "nodejs",
		"php", "php7", "php8",
		"lua", "lua5.1", "lua5.2", "lua5.3", "lua5.4",
		"awk", "gawk", "mawk", "nawk",
		"tclsh", "wish",
		"expect",
	}
	for _, interp := range interpreters {
		if base == interp {
			return true
		}
	}
	return false
}

// hasCodeExecutionFlags detects if interpreter is being invoked with code execution flags.
func hasCodeExecutionFlags(command string, args []string) bool {
	base := getBasename(command)

	// AWK special case: BEGIN/END blocks execute arbitrary code
	if isAwkWithBlocks(base, args) {
		return true
	}

	return hasDangerousFlags(base, args)
}

// isAwkWithBlocks checks for AWK commands with BEGIN/END blocks.
func isAwkWithBlocks(base string, args []string) bool {
	if base != "awk" && base != "gawk" && base != "mawk" && base != "nawk" {
		return false
	}
	for _, arg := range args {
		trimmed := strings.TrimSpace(arg)
		if strings.HasPrefix(trimmed, "BEGIN{") ||
			strings.HasPrefix(trimmed, "BEGIN {") ||
			strings.HasPrefix(trimmed, "END{") ||
			strings.HasPrefix(trimmed, "END {") {
			return true
		}
	}
	return false
}

// hasDangerousFlags checks if any arguments match dangerous flags for the given interpreter.
func hasDangerousFlags(base string, args []string) bool {
	dangerousFlags := map[string][]string{
		"python": {"-c", "--command"}, "python2": {"-c", "--command"}, "python3": {"-c", "--command"},
		"python2.7": {"-c", "--command"}, "python3.6": {"-c", "--command"}, "python3.7": {"-c", "--command"},
		"python3.8": {"-c", "--command"}, "python3.9": {"-c", "--command"}, "python3.10": {"-c", "--command"},
		"python3.11": {"-c", "--command"}, "python3.12": {"-c", "--command"},
		"perl": {"-e", "-E"}, "perl5": {"-e", "-E"},
		"ruby": {"-e"}, "irb": {"-e"},
		"node": {"-e", "--eval"}, "nodejs": {"-e", "--eval"},
		"php": {"-r"}, "php7": {"-r"}, "php8": {"-r"},
		"lua": {"-e"}, "lua5.1": {"-e"}, "lua5.2": {"-e"}, "lua5.3": {"-e"}, "lua5.4": {"-e"},
		"tclsh": {"-c"}, "wish": {"-c"},
	}

	flags, isTracked := dangerousFlags[base]
	if !isTracked {
		return false
	}

	for _, arg := range args {
		for _, flag := range flags {
			if arg == flag || strings.HasPrefix(arg, flag+"=") {
				return true
			}
		}
	}
	return false
}

// hasSuspiciousFlags detects code-execution flags in unrecognized commands.
func hasSuspiciousFlags(args []string) bool {
	suspiciousFlags := []string{"-c", "-e", "-E", "-r", "--eval", "--command"}
	for _, arg := range args {
		for _, flag := range suspiciousFlags {
			if arg == flag {
				return true
			}
		}
	}
	return false
}
