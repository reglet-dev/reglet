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
	//
	// SECURITY: Detect dangerous execution modes that allow arbitrary code execution:
	// 1. Shell execution (sh, bash, etc. with arguments)
	// 2. Interpreter code execution (python -c, perl -e, etc.)
	// 3. Unknown commands with suspicious flags (-c, -e, etc.)
	//
	// All three cases require explicit capability grant to prevent arbitrary code execution.

	// Detect shell execution
	isShell := isShellExecution(request.Command) && len(request.Args) > 0

	// Detect interpreter code execution
	isInterpreterCode := hasCodeExecutionFlags(request.Command, request.Args)

	// Detect suspicious flags in unknown commands (heuristic fallback)
	isSuspicious := !isShell && !isInterpreterCode && hasSuspiciousFlags(request.Args)

	// Any dangerous execution mode requires explicit capability
	isDangerous := isShell || isInterpreterCode || isSuspicious

	if isDangerous {
		// Determine execution type for logging and error messages
		execType := "shell"
		if isInterpreterCode {
			execType = "interpreter code execution"
		} else if isSuspicious {
			execType = "suspicious execution"
		}

		// Require explicit capability for this command
		if err := checker.Check(pluginName, "exec", request.Command); err != nil {
			errMsg := fmt.Sprintf(
				"%s requires 'exec:%s' capability (prevents arbitrary code execution)",
				execType, request.Command)
			slog.WarnContext(ctx, errMsg,
				"command", request.Command,
				"args", request.Args,
				"type", execType,
				"plugin", pluginName)
			stack[0] = hostWriteResponse(ctx, mod, ExecResponseWire{
				Error: &ErrorDetail{
					Message: errMsg,
					Type:    "capability",
				},
			})
			return
		}

		// Log successful grant of dangerous execution
		slog.InfoContext(ctx, "dangerous execution granted",
			"command", request.Command,
			"args", request.Args,
			"type", execType,
			"plugin", pluginName)
	} else {
		// Direct command execution (safe mode - no code interpretation)
		// Check capability for the specific command
		if err := checker.Check(pluginName, "exec", request.Command); err != nil {
			errMsg := fmt.Sprintf("permission denied: %v", err)
			slog.WarnContext(ctx, errMsg, "command", request.Command)
			stack[0] = hostWriteResponse(ctx, mod, ExecResponseWire{
				Error: &ErrorDetail{Message: errMsg, Type: "capability"},
			})
			return
		}
	}

	// Execute command
	// SECURITY: exec.CommandContext does NOT use shell
	// Arguments are passed directly to the binary, preventing shell injection
	// Only shell execution can interpret arguments as shell commands
	cmd := exec.CommandContext(execCtx, request.Command, request.Args...)
	if request.Dir != "" {
		cmd.Dir = request.Dir
	}

	// SECURITY: Always set cmd.Env explicitly to prevent host environment leakage
	if len(request.Env) > 0 {
		cmd.Env = request.Env
	} else {
		cmd.Env = []string{} // Explicitly empty to block environment inheritance
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

	exitCode := 0
	isTimeout := false
	var errorDetail *ErrorDetail

	if err != nil {
		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			// Other error (not found, timeout, etc.)
			errorDetail = toErrorDetail(err)
			// Map common exec errors
			if execCtx.Err() == context.DeadlineExceeded {
				errorDetail.Type = "timeout"
				errorDetail.Code = "ETIMEDOUT"
				isTimeout = true
			} else {
				errorDetail.Type = "execution"
			}
		}
	}

	if stdout.Truncated || stderr.Truncated {
		slog.WarnContext(ctx, "command output truncated",
			"command", request.Command,
			"stdout_truncated", stdout.Truncated,
			"stderr_truncated", stderr.Truncated)
	}

	slog.DebugContext(ctx, "executed command",
		"command", request.Command,
		"args", request.Args,
		"exit_code", exitCode,
		"duration", duration,
		"error", err)

	// Write response
	stack[0] = hostWriteResponse(ctx, mod, ExecResponseWire{
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		ExitCode:   exitCode,
		DurationMs: duration.Milliseconds(),
		IsTimeout:  isTimeout,
		Error:      errorDetail,
	})
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
	// Normalize to basename for matching
	base := command
	if idx := strings.LastIndex(command, "/"); idx >= 0 {
		base = command[idx+1:]
	}

	// List of common shells
	shells := []string{"sh", "bash", "dash", "zsh", "ksh", "csh", "tcsh", "fish"}
	for _, shell := range shells {
		if base == shell {
			return true
		}
	}

	return false
}

// getBasename extracts the binary name from a path.
// Examples: "/usr/bin/python" -> "python", "python3" -> "python3"
func getBasename(command string) string {
	if idx := strings.LastIndex(command, "/"); idx >= 0 {
		return command[idx+1:]
	}
	return command
}

// isKnownInterpreter detects if a command is a known scripting interpreter.
// Interpreters can execute arbitrary code similar to shells when given code-execution flags.
func isKnownInterpreter(command string) bool {
	base := getBasename(command)

	// List of known interpreters that support code execution
	interpreters := []string{
		// Python family
		"python", "python2", "python3",
		"python2.7", "python3.6", "python3.7", "python3.8", "python3.9", "python3.10", "python3.11", "python3.12",

		// Perl family
		"perl", "perl5",

		// Ruby family
		"ruby", "irb",

		// JavaScript/Node family
		"node", "nodejs",

		// PHP
		"php", "php7", "php8",

		// Lua
		"lua", "lua5.1", "lua5.2", "lua5.3", "lua5.4",

		// AWK family
		"awk", "gawk", "mawk", "nawk",

		// Other interpreters
		"tclsh", "wish", // Tcl
		"expect", // Expect (Tcl-based)
	}

	for _, interp := range interpreters {
		if base == interp {
			return true
		}
	}

	return false
}

// hasCodeExecutionFlags detects if interpreter is being invoked with flags that
// allow arbitrary code execution (similar to shell's -c flag).
//
// Examples of dangerous invocations:
//
//	python -c "import os; os.system('rm -rf /')"
//	perl -e "system('malicious')"
//	ruby -e "system 'malicious'"
//	node -e "require('child_process').exec('malicious')"
func hasCodeExecutionFlags(command string, args []string) bool {
	base := getBasename(command)

	// AWK special case: BEGIN/END blocks execute arbitrary code
	// Example: awk 'BEGIN{system("malicious")}'
	if base == "awk" || base == "gawk" || base == "mawk" || base == "nawk" {
		for _, arg := range args {
			trimmed := strings.TrimSpace(arg)
			if strings.HasPrefix(trimmed, "BEGIN{") ||
				strings.HasPrefix(trimmed, "BEGIN {") ||
				strings.HasPrefix(trimmed, "END{") ||
				strings.HasPrefix(trimmed, "END {") {
				return true
			}
		}
	}

	// Map of interpreter to their code-execution flags
	dangerousFlags := map[string][]string{
		// Python: -c "code" or --command="code"
		"python":     {"-c", "--command"},
		"python2":    {"-c", "--command"},
		"python3":    {"-c", "--command"},
		"python2.7":  {"-c", "--command"},
		"python3.6":  {"-c", "--command"},
		"python3.7":  {"-c", "--command"},
		"python3.8":  {"-c", "--command"},
		"python3.9":  {"-c", "--command"},
		"python3.10": {"-c", "--command"},
		"python3.11": {"-c", "--command"},
		"python3.12": {"-c", "--command"},

		// Perl: -e "code" or -E "code" (enhanced)
		"perl":  {"-e", "-E"},
		"perl5": {"-e", "-E"},

		// Ruby: -e "code"
		"ruby": {"-e"},
		"irb":  {"-e"},

		// Node.js: -e "code" or --eval="code"
		"node":   {"-e", "--eval"},
		"nodejs": {"-e", "--eval"},

		// PHP: -r "code" (run code without <?php tags)
		"php":  {"-r"},
		"php7": {"-r"},
		"php8": {"-r"},

		// Lua: -e "code"
		"lua":    {"-e"},
		"lua5.1": {"-e"},
		"lua5.2": {"-e"},
		"lua5.3": {"-e"},
		"lua5.4": {"-e"},

		// Tcl: -c "code"
		"tclsh": {"-c"},
		"wish":  {"-c"},
	}

	// Get dangerous flags for this interpreter
	flags, isTracked := dangerousFlags[base]
	if !isTracked {
		return false
	}

	// Check if any argument matches dangerous flags
	for _, arg := range args {
		for _, flag := range flags {
			// Match exact flag or flag with value (e.g., --command=value)
			if arg == flag || strings.HasPrefix(arg, flag+"=") {
				return true
			}
		}
	}

	return false
}

// hasSuspiciousFlags detects code-execution flags in commands we don't recognize.
// This is a fallback heuristic for lesser-known interpreters.
//
// Examples:
//
//	obscure-lang -c "malicious"  (detected even if obscure-lang isn't in our list)
//	custom-interpreter -e "code"
func hasSuspiciousFlags(args []string) bool {
	// Common code-execution flags across many interpreters
	suspiciousFlags := []string{
		"-c",        // Shell, Python, Tcl
		"-e",        // Perl, Ruby, Node, Lua
		"-E",        // Perl (enhanced)
		"-r",        // PHP
		"--eval",    // Node
		"--command", // Python
	}

	for _, arg := range args {
		for _, flag := range suspiciousFlags {
			if arg == flag {
				return true
			}
		}
	}

	return false
}
