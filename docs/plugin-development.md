# Plugin Development Guide

This guide explains how to create new WASM plugins for Reglet.

## Quick Start

Use the plugin generator to scaffold a new plugin:

```bash
./scripts/new-plugin.sh <name> --description "Plugin description" \
    --capability-kind <fs|network|exec|env> \
    --capability-pattern "<pattern>"
```

### Examples

```bash
# HTTP plugin
./scripts/new-plugin.sh http \
    --description "HTTP/HTTPS endpoint checking" \
    --capability-kind network \
    --capability-pattern "outbound:80,443,*"

# DNS plugin
./scripts/new-plugin.sh dns \
    --description "DNS resolution validation" \
    --capability-kind network \
    --capability-pattern "outbound:53"

# Process plugin
./scripts/new-plugin.sh process \
    --description "Running process checks" \
    --capability-kind fs \
    --capability-pattern "read:/proc/**"
```

## Plugin Structure

A plugin consists of:

```
plugins/<name>/
├── main.go      # Plugin implementation
├── Makefile     # Build configuration
└── README.md    # Documentation
```

## Required Exports

Every plugin MUST export these five functions:

### 1. Memory Management

```go
//go:wasmexport allocate
func allocate(size uint32) uint32

//go:wasmexport deallocate
func deallocate(ptr uint32, size uint32)
```

**Critical**: These functions implement the memory pinning pattern that prevents
garbage collection corruption. The template provides the correct implementation.

### 2. Plugin Metadata

```go
//go:wasmexport describe
func describe() uint32
```

Returns JSON with plugin name, version, description, and capabilities.

### 3. Configuration Schema

```go
//go:wasmexport schema
func schema() uint32
```

Returns JSON Schema defining valid configuration fields.

### 4. Observation Execution

```go
//go:wasmexport observe
func observe(configPtr uint32, configLen uint32) uint32
```

Main entry point - reads config, performs the check, returns results.

## Helper Functions

The template provides these helper functions:

```go
// Memory operations (WASM-specific, uses unsafe.Pointer)
copyToMemory(ptr uint32, data []byte)
readFromMemory(ptr uint32, length uint32) []byte

// JSON marshaling
marshalToPtr(data interface{}) uint32

// Response builders
successResponse(data map[string]interface{}) uint32
errorResponse(message string) uint32
```

## Implementation Workflow

### 1. Generate Plugin Scaffold

```bash
./scripts/new-plugin.sh myplugin --description "My plugin"
cd plugins/myplugin
```

### 2. Define Configuration Schema

Edit `schema()` function:

```go
func schema() uint32 {
    configSchema := map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "url": map[string]interface{}{
                "type":        "string",
                "description": "URL to check",
            },
            "timeout": map[string]interface{}{
                "type":        "integer",
                "description": "Timeout in seconds",
                "default":     30,
            },
        },
        "required": []string{"url"},
    }
    return marshalToPtr(configSchema)
}
```

### 3. Implement Observation Logic

Edit `observe()` function:

```go
func observe(configPtr uint32, configLen uint32) uint32 {
    // 1. Read config
    configData := readFromMemory(configPtr, configLen)
    var config map[string]interface{}
    if err := json.Unmarshal(configData, &config); err != nil {
        return errorResponse(fmt.Sprintf("invalid config: %v", err))
    }

    // 2. Extract and validate fields
    url, ok := config["url"].(string)
    if !ok || url == "" {
        return errorResponse("missing required field: url")
    }

    timeout := 30 // default
    if t, ok := config["timeout"].(float64); ok {
        timeout = int(t)
    }

    // 3. Perform your check/validation
    result, err := doYourCheck(url, timeout)
    if err != nil {
        return errorResponse(fmt.Sprintf("check failed: %v", err))
    }

    // 4. Return success response with evidence
    return successResponse(map[string]interface{}{
        "url":           url,
        "response_code": result.Code,
        "response_time": result.TimeMs,
    })
}
```

### 4. Build and Test

```bash
# Build to WASM
make build

# Test loading in integration tests
# See internal/infrastructure/wasm/plugin_integration_test.go for examples
```

## Memory Management Pattern

**CRITICAL**: The memory pinning pattern is essential for WASM plugins.

### The Problem

When a Go WASM plugin allocates memory and returns a pointer to the host,
the Go garbage collector doesn't know the host still needs that memory.
Without keeping a reference, the GC will reclaim it, causing corruption.

### The Solution

```go
// Global map "pins" memory by keeping references
var allocations = make(map[uint32][]byte)

func allocate(size uint32) uint32 {
    buf := make([]byte, size)
    ptr := uint32(uintptr(unsafe.Pointer(&buf[0])))

    // PIN: Store slice in map so GC sees it as "in use"
    allocations[ptr] = buf

    return ptr
}

func deallocate(ptr uint32, size uint32) {
    // UNPIN: Remove from map, allowing GC to collect
    delete(allocations, ptr)
}
```

### Memory Lifecycle

```
1. Plugin calls allocate() → returns ptr
2. Plugin stores data in allocations[ptr] (pinned)
3. Plugin writes JSON to ptr via copyToMemory()
4. Plugin returns ptr to host
5. Host reads JSON from ptr
6. Host calls deallocate(ptr)
7. Plugin removes allocations[ptr] (unpinned)
8. GC can now collect the memory
```

## Response Format

### Success Response

```json
{
  "status": true,
  "field1": "value",
  "field2": 123,
  "field3": true
}
```

### Error Response

```json
{
  "status": false,
  "error": "error message here"
}
```

## Capabilities

Capabilities declare what resources the plugin needs:

- **`fs:read:<pattern>`** - Read filesystem access
- **`fs:write:<pattern>`** - Write filesystem access
- **`network:outbound:<ports>`** - Network access
- **`exec:<commands>`** - Command execution
- **`env:<pattern>`** - Environment variables

Examples:

```go
"capabilities": []map[string]string{
    {"kind": "fs", "pattern": "read:/etc/**"},           // Read /etc
    {"kind": "network", "pattern": "outbound:80,443"},   // HTTP/HTTPS
    {"kind": "network", "pattern": "outbound:*"},        // Any port
    {"kind": "exec", "pattern": "systemctl"},            // Run systemctl
    {"kind": "env", "pattern": "AWS_*"},                 // AWS env vars
}
```

## Testing

### Unit Testing

Plugin logic can be unit tested as normal Go code before compilation to WASM.

### Integration Testing

Add tests to `internal/infrastructure/wasm/plugin_integration_test.go`:

```go
func Test_MyPlugin_Success(t *testing.T) {
    ctx := context.Background()
    runtime, err := wasm.NewRuntime(ctx)
    require.NoError(t, err)
    defer runtime.Close()

    // Load plugin WASM
    wasmBytes, err := os.ReadFile("../../plugins/myplugin/myplugin.wasm")
    require.NoError(t, err)

    plugin, err := runtime.LoadPlugin("myplugin", wasmBytes)
    require.NoError(t, err)

    // Test describe()
    info, err := plugin.Describe()
    require.NoError(t, err)
    assert.Equal(t, "myplugin", info.Name)

    // Test observe() - success case
    config := wasm.Config{
        Values: map[string]string{
            "url": "https://example.com",
        },
    }
    result, err := plugin.Observe(config)
    require.NoError(t, err)
    assert.True(t, result.Evidence.Data["status"].(bool))
}
```

## Build System

### Building Single Plugin

```bash
cd plugins/myplugin
make build
```

### Building All Plugins

```bash
make build-plugins       # From project root
```

### Build Command

```bash
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o plugin.wasm main.go
```

- `GOOS=wasip1` - WebAssembly System Interface (POSIX subset)
- `GOARCH=wasm` - WebAssembly architecture
- `-buildmode=c-shared` - Required for WASI
- Output: ~3-4MB per plugin (acceptable for embedding)

## Common Patterns

### Reading Configuration

```go
// String field
url, ok := config["url"].(string)
if !ok || url == "" {
    return errorResponse("missing required field: url")
}

// Integer field with default
timeout := 30
if t, ok := config["timeout"].(float64); ok {
    timeout = int(t) // JSON numbers are float64
}

// Boolean field
verify := true
if v, ok := config["verify_tls"].(bool); ok {
    verify = v
}

// Array field
var hosts []string
if h, ok := config["hosts"].([]interface{}); ok {
    for _, item := range h {
        if s, ok := item.(string); ok {
            hosts = append(hosts, s)
        }
    }
}
```

### Error Handling

```go
result, err := someOperation()
if err != nil {
    return errorResponse(fmt.Sprintf("operation failed: %v", err))
}

// Validation error
if value < 0 {
    return errorResponse(fmt.Sprintf("invalid value: %d (must be >= 0)", value))
}
```

### Timing Operations

```go
import "time"

start := time.Now()
result, err := doSomething()
duration := time.Since(start)

return successResponse(map[string]interface{}{
    "result":      result,
    "duration_ms": duration.Milliseconds(),
})
```

## Best Practices

1. **Validate all config fields** before use
2. **Return specific error messages** - help users fix their profiles
3. **Include relevant evidence** - timestamps, response codes, sizes, etc.
4. **Use timeouts** for network/exec operations
5. **Handle edge cases** - empty strings, nil values, out-of-range numbers
6. **Document your schema** in README.md with examples
7. **Test both success and failure paths**
8. **Keep plugins focused** - one responsibility per plugin

## Network Operations

**IMPORTANT**: WASI Preview 1 does not support network sockets directly. Network operations use **host functions** provided by Reglet.

### Available Host Functions

| Plugin | Host Function | Status |
|:-------|:--------------|:-------|
| **dns** | `dns_resolve` | ✅ Complete |
| **http** | `http_request` | ✅ Complete |
| **tcp** | `tcp_connect` | ✅ Complete |
| **smtp** | `smtp_connect` | ✅ Complete |

### How It Works

1. Plugin calls the host function (e.g., `dns_resolve`)
2. Host performs the network operation with capability checking
3. Results are marshaled back to the plugin via shared memory

See existing network plugins (`plugins/dns/`, `plugins/http/`, `plugins/tcp/`, `plugins/smtp/`) for implementation examples.

## Platform Considerations

### Linux-Specific Plugins

Some plugins only work on Linux (process, systemd). Document this clearly:

```go
// In describe()
"platform": "linux",
```

In tests, skip gracefully:

```go
if runtime.GOOS != "linux" {
    t.Skip("process plugin is Linux-only")
}
```

### Cross-Platform Plugins

Network plugins (http, tcp, dns) work on all platforms.

## References

- **File Plugin**: `plugins/file/main.go` - Reference implementation
- **Memory Guide**: `docs/plugin-memory-management.md` - Deep dive on memory
- **Integration Tests**: `internal/infrastructure/wasm/plugin_integration_test.go` - Test patterns
- **Host Interface**: `internal/infrastructure/wasm/plugin.go` - How host calls plugins
