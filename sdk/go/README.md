# Reglet Go SDK

The Reglet Go SDK provides Go APIs for writing WebAssembly (WASM) plugins for the Reglet compliance platform. It handles memory management, host communication, plugin registration, and provides safe wrappers for network operations, command execution, and logging.

## Version

**Current Version**: `0.1.0-alpha`
**Minimum Host Version**: `0.2.0`

## Features

- ✅ **Full Context Propagation**: Deadlines, cancellation, and values flow to all operations
- ✅ **Memory Management**: Automatic allocation tracking with 100 MB safety limit
- ✅ **Network Operations**: DNS, HTTP, and TCP with explicit API
- ✅ **Command Execution**: Sandboxed command execution via host
- ✅ **Structured Logging**: Go `slog` integration with host-side collection
- ✅ **Type-Safe Wire Protocol**: JSON-based ABI with validation
- ✅ **Comprehensive Testing**: 85%+ test coverage

## Installation

```bash
go get github.com/whiskeyjimbo/reglet/sdk
```

## Quick Start

### Minimal Plugin

```go
package main

import (
    "context"
    "log/slog"

    sdk "github.com/whiskeyjimbo/reglet/sdk"
    _ "github.com/whiskeyjimbo/reglet/sdk/log" // Initialize logging
)

type MyPlugin struct{}

func main() {
    sdk.Register(&MyPlugin{})
}

func (p *MyPlugin) Describe(ctx context.Context) (sdk.Metadata, error) {
    return sdk.Metadata{
        Name:         "my-plugin",
        Version:      "1.0.0",
        Description:  "Example compliance check plugin",
        Capabilities: []sdk.Capability{
            {Kind: "network:outbound", Pattern: "example.com:443"},
        },
    }, nil
}

func (p *MyPlugin) Schema(ctx context.Context) ([]byte, error) {
    return sdk.GenerateSchema(struct {
        Hostname string `json:"hostname" description:"Hostname to check"`
    }{})
}

func (p *MyPlugin) Check(ctx context.Context, config sdk.Config) (sdk.Evidence, error) {
    hostname, err := sdk.MustGetString(config, "hostname")
    if err != nil {
        return sdk.Failure("config", err.Error()), nil
    }

    slog.InfoContext(ctx, "Starting check", "hostname", hostname)

    // Plugin logic here...

    return sdk.Success(map[string]interface{}{
        "hostname": hostname,
        "status":   "ok",
    }), nil
}
```

### Building

```bash
GOOS=wasip1 GOARCH=wasm go build -o plugin.wasm main.go
```

## Package Documentation

Detailed documentation for each subpackage:

- **[exec](exec/README.md)** - Command execution
- **[log](log/README.md)** - Structured logging
- **[net](net/README.md)** - Network operations (DNS, HTTP, TCP)
- **[internal/abi](internal/abi/README.md)** - WASM ABI and memory management (internal)

## Core Concepts

### Plugin Interface

Every plugin must implement three methods:

```go
type Plugin interface {
    // Describe returns metadata about the plugin
    Describe(ctx context.Context) (Metadata, error)

    // Schema returns JSON schema for plugin configuration
    Schema(ctx context.Context) ([]byte, error)

    // Check executes the main plugin logic
    Check(ctx context.Context, config Config) (Evidence, error)
}
```

### Context Propagation

All SDK functions properly propagate Go contexts:

```go
// Timeouts
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
resp, err := net.Get(ctx, url) // Respects 5 second timeout

// Cancellation
ctx, cancel := context.WithCancel(context.Background())
go func() {
    time.Sleep(1 * time.Second)
    cancel() // Cancels the HTTP request
}()
resp, err := net.Get(ctx, url)

// Values (for request tracing)
ctx = context.WithValue(ctx, "request_id", "abc123")
resp, err := net.Get(ctx, url) // request_id passed to host logs
```

### Memory Management

The SDK tracks all memory allocations and enforces a **100 MB limit**:

```go
const MaxTotalAllocations = 100 * 1024 * 1024 // 100 MB
```

If your plugin exceeds this limit, it will panic with:
```
abi: memory allocation limit exceeded (requested: X bytes, current: Y bytes, limit: 104857600 bytes)
```

**Best Practices:**
- Stream large data instead of loading into memory
- Free resources promptly (close HTTP response bodies)
- Avoid caching large datasets in plugin memory

### Version Checking

The SDK automatically reports its version in plugin metadata:

```go
metadata, _ := plugin.Describe(ctx)
// metadata.SDKVersion = "0.1.0-alpha"
// metadata.MinHostVersion = "0.2.0"
```

The **host is responsible** for validating compatibility:
- If host version < MinHostVersion → reject plugin
- If plugin uses unsupported SDK features → runtime errors

## Network Operations

### DNS Resolution

Use the `WasmResolver` for DNS lookups:

```go
import sdknet "github.com/whiskeyjimbo/reglet/sdk/net"

resolver := &sdknet.WasmResolver{
    Nameserver: "", // Empty = use host's default
}
ips, err := resolver.LookupHost(ctx, "example.com")
```

See [net/README.md](net/README.md) for full DNS API documentation.

### HTTP Requests

**Option 1 - SDK Helpers (Recommended):**
```go
import sdknet "github.com/whiskeyjimbo/reglet/sdk/net"

resp, err := sdknet.Get(ctx, "https://example.com")
defer resp.Body.Close()
```

**Option 2 - Custom Client:**
```go
import (
    "net/http"
    sdknet "github.com/whiskeyjimbo/reglet/sdk/net"
)

client := &http.Client{
    Transport: &sdknet.WasmTransport{},
    Timeout:   10 * time.Second,
}
resp, err := client.Get("https://example.com")
```

#### HTTP Body Size Limit

HTTP response bodies are limited to **10 MB** (`net.MaxHTTPBodySize`).

See [net/README.md](net/README.md) for full HTTP API documentation.

### TCP Connections

```go
import sdknet "github.com/whiskeyjimbo/reglet/sdk/net"

// DialTCP(ctx, host, port, timeoutMs, useTLS)
conn, err := sdknet.DialTCP(ctx, "example.com", "443", 5000, true)
if err != nil {
    return sdk.Failure("tcp", err.Error()), nil
}

return sdk.Success(map[string]interface{}{
    "connected":      conn.Connected,
    "tls":            conn.TLS,
    "tls_version":    conn.TLSVersion,
    "response_ms":    conn.ResponseTimeMs,
}), nil
```

See [net/README.md](net/README.md) for full TCP API documentation.

## Command Execution

Execute host commands via sandboxed interface:

```go
import "github.com/whiskeyjimbo/reglet/sdk/exec"

req := exec.CommandRequest{
    Command: "systemctl",
    Args:    []string{"is-active", "nginx"},
}

result, err := exec.Run(ctx, req)
if err != nil {
    return sdk.Failure("exec", err.Error()), nil
}

serviceActive := result.ExitCode == 0

return sdk.Success(map[string]interface{}{
    "service": "nginx",
    "active":  serviceActive,
    "stdout":  result.Stdout,
}), nil
```

See [exec/README.md](exec/README.md) for full exec API documentation.

## Structured Logging

Use Go's standard `log/slog` package:

```go
import (
    "log/slog"
    _ "github.com/whiskeyjimbo/reglet/sdk/log" // Initialize WASM logging
)

func (p *MyPlugin) Check(ctx context.Context, config sdk.Config) (sdk.Evidence, error) {
    // Context-aware logging (recommended)
    slog.InfoContext(ctx, "Starting check", "config_keys", len(config))

    // Structured attributes
    slog.Debug("Processing item", "item_id", 123, "status", "pending")

    // Error logging
    if err != nil {
        slog.ErrorContext(ctx, "Operation failed", "error", err, "retry_count", 3)
    }

    return sdk.Success(nil), nil
}
```

See [log/README.md](log/README.md) for full logging API documentation.

## Schema Generation

Generate JSON Schema from Go structs:

```go
type PluginConfig struct {
    Hostname string `json:"hostname" description:"Target hostname"`
    Port     int    `json:"port" default:"443" description:"Target port"`
    Timeout  int    `json:"timeout,omitempty" description:"Request timeout in seconds"`
}

func (p *MyPlugin) Schema(ctx context.Context) ([]byte, error) {
    return sdk.GenerateSchema(PluginConfig{})
}
```

**Supported Tags:**
- `json:"field_name"` - Field name in schema
- `json:",omitempty"` - Field is optional
- `description:"..."` - Field description
- `default:"..."` - Default value

## Error Handling

Use SDK error helpers for consistent error reporting:

```go
// Success
return sdk.Success(map[string]interface{}{
    "result": "ok",
}), nil

// Simple failure
return sdk.Failure("validation", "Invalid hostname format"), nil

// Network error
return sdk.Evidence{
    Status: false,
    Error: sdk.ToErrorDetail(&sdk.NetworkError{
        Operation: "connect",
        Target:    "example.com:443",
        Err:       err,
    }),
}, nil

// Configuration error
return sdk.Evidence{
    Status: false,
    Error: sdk.ToErrorDetail(&sdk.ConfigError{
        Field: "hostname",
        Err:   fmt.Errorf("missing required field"),
    }),
}, nil
```

## Capabilities

Request capabilities in `Describe()`:

```go
func (p *MyPlugin) Describe(ctx context.Context) (sdk.Metadata, error) {
    return sdk.Metadata{
        Name:    "my-plugin",
        Version: "1.0.0",
        Capabilities: []sdk.Capability{
            // Network access to specific host
            {Kind: "network:outbound", Pattern: "api.example.com:443"},

            // DNS resolution
            {Kind: "network:dns", Pattern: "*"},

            // Command execution
            {Kind: "exec", Pattern: "systemctl"},

            // Filesystem read
            {Kind: "fs:read", Pattern: "/etc/nginx/*.conf"},
        },
    }, nil
}
```

**Note**: Capabilities are granted by the host via system configuration, not by the plugin itself.

## Limitations

### Network

- **No Streaming**: HTTP responses are fully buffered (10 MB limit)
- **No WebSockets**: Not supported
- **TCP Check-Only**: Can connect but not perform bidirectional communication
- **No UDP**: UDP protocol not supported
- **No Raw Sockets**: Only standard protocols (HTTP, TCP, DNS)

### Execution

- **Single-Threaded**: WASI Preview 1 is single-threaded (use goroutines for logical concurrency)
- **No Interactive Commands**: Commands requiring stdin will fail
- **No Shell Features**: Commands executed directly, not through a shell

### Filesystem

- **Sandboxed Access**: Restricted to paths granted by capabilities
- **No Direct WASI**: Must use host functions, not WASI filesystem directly

### Memory

- **100 MB Limit**: Total allocations capped at 100 MB
- **No Memory Pooling**: Each allocation goes through Go's allocator

## Best Practices

### 1. Always Use Context with Timeouts

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// All operations respect the timeout
resp, err := sdknet.Get(ctx, url)
result, err := exec.Run(ctx, req)
ips, err := resolver.LookupHost(ctx, hostname)
```

### 2. Close HTTP Response Bodies

```go
resp, err := sdknet.Get(ctx, url)
if err != nil {
    return sdk.Failure("http", err.Error()), nil
}
defer resp.Body.Close() // ✅ Always defer close

body, _ := io.ReadAll(resp.Body)
```

### 3. Use Structured Logging

```go
// ❌ Bad
slog.Info(fmt.Sprintf("User %s logged in", userID))

// ✅ Good
slog.Info("User logged in", "user_id", userID)
```

### 4. Handle Expected Errors Gracefully

```go
result, err := exec.Run(ctx, req)
if err != nil {
    // Unexpected error (command not found, permission denied)
    return sdk.Failure("exec", err.Error()), nil
}

if result.ExitCode != 0 {
    // Expected non-zero exit (command ran but failed)
    return sdk.Success(map[string]interface{}{
        "check_passed": false,
        "exit_code":    result.ExitCode,
        "stderr":       result.Stderr,
    }), nil
}
```

### 5. Request Minimal Capabilities

```go
// ❌ Too broad
{Kind: "network:outbound", Pattern: "*"}

// ✅ Specific
{Kind: "network:outbound", Pattern: "api.example.com:443"}
```

## Testing Plugins

### Unit Testing

```go
func TestMyPlugin_Check(t *testing.T) {
    plugin := &MyPlugin{}
    config := sdk.Config{
        "hostname": "example.com",
    }

    ctx := context.Background()
    evidence, err := plugin.Check(ctx, config)

    require.NoError(t, err)
    assert.True(t, evidence.Status)
}
```

### Integration Testing

```bash
# Build WASM plugin
GOOS=wasip1 GOARCH=wasm go build -o plugin.wasm

# Run with reglet CLI
reglet check --profile test-profile.yaml
```

## Troubleshooting

### "memory allocation limit exceeded"

**Cause**: Your plugin exceeded the 100 MB memory limit.

**Solutions:**
- Stream data instead of loading into memory
- Close HTTP response bodies promptly
- Avoid caching large datasets
- Process data in chunks

### "HTTP response body exceeds maximum size"

**Cause**: Response body > 10 MB.

**Solutions:**
- Request smaller data chunks (use pagination)
- Stream responses if host supports it
- Compress responses at source

### "context deadline exceeded"

**Cause**: Operation took longer than context timeout.

**Solutions:**
- Increase context timeout
- Optimize slow operations
- Use concurrent requests (via goroutines)

## Config Helpers

The SDK provides safe config extraction functions to prevent panics from direct type assertions:

```go
// Required fields - returns error if missing
hostname, err := sdk.MustGetString(config, "hostname")
port, err := sdk.MustGetInt(config, "port")
enabled, err := sdk.MustGetBool(config, "enabled")

// Optional fields with defaults
timeout := sdk.GetIntDefault(config, "timeout", 30)
protocol := sdk.GetStringDefault(config, "protocol", "https")
verbose := sdk.GetBoolDefault(config, "verbose", false)

// Safe extraction (returns ok=false if missing or wrong type)
value, ok := sdk.GetString(config, "optional_field")
number, ok := sdk.GetInt(config, "optional_number")
tags, ok := sdk.GetStringSlice(config, "tags")
```

## Examples

See the [examples directory](examples/) for complete working plugins:

- **[basic-plugin](examples/basic-plugin/)** - HTTP endpoint checker demonstrating SDK patterns

## Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md) for development guidelines.

## License

See [LICENSE](../../LICENSE) for details.
