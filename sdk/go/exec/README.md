# Exec Package

The `exec` package provides command execution capabilities for Reglet WASM plugins. It allows plugins to execute shell commands on the host system through a sandboxed interface.

## Overview

This package wraps the host's command execution functionality, translating Go-style command requests into wire format messages that cross the WASM boundary. All command execution happens on the host side with explicit capability grants.

## Security Model

- **Requires Capability**: `exec` or `exec:<pattern>` capability grant
- **Sandboxed**: Commands run in host-controlled environment
- **No Direct Access**: Plugin cannot directly access host filesystem or processes
- **Configurable Limits**: Host enforces timeouts, output size limits, and allowed commands

## Basic Usage

```go
package main

import (
    "context"
    "log"

    "github.com/whiskeyjimbo/reglet/sdk"
    "github.com/whiskeyjimbo/reglet/sdk/exec"
)

type MyPlugin struct{}

func (p *MyPlugin) Check(ctx context.Context, config sdk.Config) (sdk.Evidence, error) {
    // Simple command execution
    req := exec.Request{
        Command: "ls",
        Args:    []string{"-la", "/tmp"},
    }

    result, err := exec.Run(ctx, req)
    if err != nil {
        return sdk.Failure("exec", err.Error()), nil
    }

    return sdk.Success(map[string]interface{}{
        "stdout":    result.Stdout,
        "stderr":    result.Stderr,
        "exit_code": result.ExitCode,
        "duration":  result.Duration,
    }), nil
}
```

## Advanced Usage

### Environment Variables

```go
req := exec.Request{
    Command: "env",
    Env: map[string]string{
        "MY_VAR":    "value",
        "PATH":      "/usr/local/bin:/usr/bin:/bin",
    },
}

result, err := exec.Run(ctx, req)
```

### Working Directory

```go
req := exec.Request{
    Command: "pwd",
    Dir:     "/var/log",  // Run command in specific directory
}

result, err := exec.Run(ctx, req)
```

### Timeout Handling

Use Go's context to enforce timeouts:

```go
// 5 second timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

req := exec.Request{
    Command: "sleep",
    Args:    []string{"10"},
}

result, err := exec.Run(ctx, req)
if err != nil {
    // err will be timeout error if command exceeds 5 seconds
}
```

### Exit Code Handling

```go
req := exec.Request{
    Command: "grep",
    Args:    []string{"pattern", "file.txt"},
}

result, err := exec.Run(ctx, req)
if err != nil {
    return sdk.Failure("exec", err.Error()), nil
}

// Check exit code
if result.ExitCode != 0 {
    return sdk.Success(map[string]interface{}{
        "found":     false,
        "exit_code": result.ExitCode,
    }), nil
}

return sdk.Success(map[string]interface{}{
    "found":  true,
    "output": result.Stdout,
}), nil
```

## API Reference

### Request

```go
type Request struct {
    Command string            // Command to execute (required)
    Args    []string          // Command arguments (optional)
    Dir     string            // Working directory (optional, defaults to host's choice)
    Env     map[string]string // Environment variables (optional)
}
```

### Response

```go
type Response struct {
    Stdout   string        // Standard output from command
    Stderr   string        // Standard error from command
    ExitCode int           // Exit code (0 = success)
    Duration time.Duration // How long the command took to execute
}
```

### Functions

#### Run

```go
func Run(ctx context.Context, req Request) (*Response, error)
```

Executes a command on the host system. Returns the command output and metadata, or an error if the command cannot be executed.

**Errors:**
- `context.DeadlineExceeded`: Command exceeded timeout
- Capability errors: Plugin lacks required permissions
- Execution errors: Command not found, permission denied, etc.

## Common Patterns

### Checking Service Status

```go
req := exec.Request{
    Command: "systemctl",
    Args:    []string{"is-active", "nginx"},
}

result, err := exec.Run(ctx, req)
if err != nil {
    return sdk.Failure("exec", err.Error()), nil
}

isActive := result.ExitCode == 0

return sdk.Success(map[string]interface{}{
    "service": "nginx",
    "active":  isActive,
}), nil
```

### File Validation

```go
req := exec.Request{
    Command: "test",
    Args:    []string{"-f", "/etc/passwd"},
}

result, err := exec.Run(ctx, req)
if err != nil {
    return sdk.Failure("exec", err.Error()), nil
}

fileExists := result.ExitCode == 0
```

### Parsing Command Output

```go
req := exec.Request{
    Command: "df",
    Args:    []string{"-h", "/"},
}

result, err := exec.Run(ctx, req)
if err != nil {
    return sdk.Failure("exec", err.Error()), nil
}

// Parse the output
lines := strings.Split(result.Stdout, "\n")
// ... process lines
```

## Limitations

1. **No Shell Features**: Commands are executed directly, not through a shell. Use explicit commands instead of shell syntax:
   - ❌ `"ls | grep foo"`
   - ✅ `exec.Run()` with `"grep"` and pass result of `"ls"` as input

2. **No Interactive Commands**: Commands requiring stdin interaction will hang or fail

3. **Output Size Limits**: Host may truncate very large output (typically 10MB limit)

4. **Command Whitelist**: Host may restrict which commands can be executed via capability patterns

5. **Working Directory**: Host may restrict which directories can be used as working directory

## Best Practices

1. **Use Timeouts**: Always use context with timeout for long-running commands
2. **Check Exit Codes**: Don't just check for errors, verify exit codes for command success
3. **Validate Input**: Sanitize any user-provided input used in commands
4. **Handle Output Size**: Be prepared for truncated output on very large results
5. **Specify Full Paths**: Use absolute paths for commands when possible (`/usr/bin/ls` vs `ls`)
6. **Request Minimal Capabilities**: Only request exec capabilities for commands you actually need

## Wire Format

The exec package uses JSON-based wire format to communicate with the host:

```go
// Request sent to host
{
    "context": { /* context metadata */ },
    "command": "ls",
    "args": ["-la"],
    "dir": "/tmp",
    "env": {"PATH": "..."}
}

// Response from host
{
    "stdout": "...",
    "stderr": "...",
    "exit_code": 0,
    "duration": "100ms",
    "error": null
}
```

## Context Propagation

The exec package fully supports Go context propagation:

- **Cancellation**: Cancelled context terminates the running command
- **Deadlines**: Context deadlines enforce command timeouts
- **Values**: Context values (like request IDs) are passed to host

## See Also

- [Main SDK Documentation](../README.md)
- [Plugin Development Guide](../../docs/plugin-development.md)
- [Capability System](../../docs/capabilities.md)
