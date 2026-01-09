# Plugin Development Guide

This guide explains how to create WASM plugins for Reglet using the Reglet SDK.

## Quick Start

### 1. Create Plugin Directory

```bash
mkdir -p plugins/myplugin
cd plugins/myplugin
```

### 2. Initialize Go Module

```bash
go mod init github.com/yourorg/reglet-plugins/myplugin
go get github.com/reglet-dev/reglet/sdk@latest
```

### 3. Create Plugin

```go
// plugin.go
package main

import (
    "context"
    
    sdk "github.com/reglet-dev/reglet/sdk"
)

type myPlugin struct{}

func (p *myPlugin) Describe(ctx context.Context) (sdk.Metadata, error) {
    return sdk.Metadata{
        Name:        "myplugin",
        Version:     "1.0.0",
        Description: "My custom plugin",
        Capabilities: []sdk.Capability{
            {Kind: "fs", Pattern: "read:/etc/**"},
        },
    }, nil
}

func (p *myPlugin) Schema(ctx context.Context) ([]byte, error) {
    return sdk.GenerateSchema(MyConfig{})
}

func (p *myPlugin) Check(ctx context.Context, config sdk.Config) (sdk.Evidence, error) {
    var cfg MyConfig
    if err := sdk.ValidateConfig(config, &cfg); err != nil {
        return sdk.Evidence{Status: false, Error: sdk.ToErrorDetail(err)}, nil
    }
    
    // Your logic here
    return sdk.Success(map[string]interface{}{
        "result": "ok",
    }), nil
}

type MyConfig struct {
    Path string `json:"path" validate:"required"`
}

func init() {
    sdk.Register(&myPlugin{})
}

func main() {} // Required but never called
```

### 4. Build WASM

```bash
GOOS=wasip1 GOARCH=wasm go build -o myplugin.wasm .
```

## Plugin Interface

Every plugin must implement the `sdk.Plugin` interface:

```go
type Plugin interface {
    Describe(ctx context.Context) (Metadata, error)
    Schema(ctx context.Context) ([]byte, error)
    Check(ctx context.Context, config Config) (Evidence, error)
}
```

### Describe

Returns plugin metadata and required capabilities:

```go
func (p *myPlugin) Describe(ctx context.Context) (sdk.Metadata, error) {
    return sdk.Metadata{
        Name:        "myplugin",
        Version:     "1.0.0",
        Description: "What this plugin does",
        Capabilities: []sdk.Capability{
            {Kind: "fs", Pattern: "read:/etc/**"},
            {Kind: "network", Pattern: "outbound:443"},
        },
    }, nil
}
```

### Schema

Returns JSON Schema for configuration validation:

```go
type MyConfig struct {
    URL     string `json:"url" validate:"required" description:"Target URL"`
    Timeout int    `json:"timeout,omitempty" description:"Timeout in seconds"`
}

func (p *myPlugin) Schema(ctx context.Context) ([]byte, error) {
    return sdk.GenerateSchema(MyConfig{})
}
```

### Check

Executes the plugin logic and returns evidence:

```go
func (p *myPlugin) Check(ctx context.Context, config sdk.Config) (sdk.Evidence, error) {
    // 1. Parse and validate config
    var cfg MyConfig
    if err := sdk.ValidateConfig(config, &cfg); err != nil {
        return sdk.Evidence{
            Status: false,
            Error:  sdk.ToErrorDetail(&sdk.ConfigError{Err: err}),
        }, nil
    }
    
    // 2. Perform your check
    result, err := doCheck(cfg)
    if err != nil {
        return sdk.Failure("internal", err.Error()), nil
    }
    
    // 3. Return success with evidence
    return sdk.Success(map[string]interface{}{
        "url":        cfg.URL,
        "status":     result.Status,
        "latency_ms": result.LatencyMs,
    }), nil
}
```

## SDK Functions

### Config Validation

```go
// Parse config map into typed struct
var cfg MyConfig
if err := sdk.ValidateConfig(config, &cfg); err != nil {
    return sdk.Evidence{Status: false, Error: sdk.ToErrorDetail(err)}, nil
}
```

### Schema Generation

```go
// Generate JSON Schema from struct tags
type Config struct {
    Path    string `json:"path" validate:"required" description:"File path"`
    Timeout int    `json:"timeout,omitempty" description:"Timeout in seconds"`
}

schema, err := sdk.GenerateSchema(Config{})
```

### Response Builders

```go
// Success with data
return sdk.Success(map[string]interface{}{
    "exists": true,
    "size":   1024,
}), nil

// Failure with typed error
return sdk.Failure("network", "connection refused"), nil

// Failure with error detail
return sdk.Evidence{
    Status: false,
    Error:  sdk.ToErrorDetail(err),
}, nil
```

### Error Types

Use typed errors for better error categorization:

```go
// Config errors
&sdk.ConfigError{Field: "path", Err: errors.New("path is required")}

// Network errors
&sdk.NetworkError{Operation: "connect", Host: "example.com", Err: err}

// Timeout errors
&sdk.TimeoutError{Operation: "http_request", Duration: 30*time.Second}

// Capability errors
&sdk.CapabilityError{Required: "fs:read:/etc/passwd"}
```

## Capabilities

Capabilities declare what resources the plugin needs:

| Kind | Pattern | Example |
|:-----|:--------|:--------|
| `fs` | `read:<path>` | `read:/etc/**` |
| `fs` | `write:<path>` | `write:/tmp/**` |
| `network` | `outbound:<ports>` | `outbound:80,443` |
| `exec` | `<command>` | `systemctl` |
| `env` | `<pattern>` | `AWS_*` |

## Plugin Structure

```
plugins/myplugin/
├── plugin.go     # Main plugin implementation
├── go.mod        # Go module
├── Makefile      # Build configuration (optional)
└── README.md     # Documentation
```

## Build Commands

```bash
# Build single plugin
GOOS=wasip1 GOARCH=wasm go build -o myplugin.wasm .

# Build all plugins (from project root)
make build-plugins
```

## Testing

### Unit Tests

Test your plugin logic as normal Go code:

```go
func TestMyPlugin_Check(t *testing.T) {
    p := &myPlugin{}
    ctx := context.Background()
    
    config := sdk.Config{"url": "https://example.com"}
    evidence, err := p.Check(ctx, config)
    
    require.NoError(t, err)
    assert.True(t, evidence.Status)
}
```

### Integration Tests

See `internal/infrastructure/wasm/plugin_integration_test.go` for WASM integration test examples.

## Network Operations

WASI doesn't support direct network sockets. Network plugins use **host functions**:

| Plugin | Host Function | Status |
|:-------|:--------------|:-------|
| `dns` | `dns_resolve` | ✅ |
| `http` | `http_request` | ✅ |
| `tcp` | `tcp_connect` | ✅ |
| `smtp` | `smtp_connect` | ✅ |

See `sdk/go/net/` for the SDK network client implementations.

## Best Practices

1. **Use typed configs** - Define struct with `json` and `validate` tags
2. **Validate early** - Call `sdk.ValidateConfig` at start of Check()
3. **Return specific errors** - Use typed error types for categorization
4. **Include evidence** - Return useful data for expect expressions
5. **Handle timeouts** - Use context for cancellation
6. **Keep plugins focused** - One responsibility per plugin

## References

- **File Plugin**: `plugins/file/plugin.go` - Reference implementation
- **SDK Types**: `sdk/go/types.go` - Core types (Evidence, Config, Metadata)
- **SDK Helpers**: `sdk/go/helpers.go` - ValidateConfig, GenerateSchema
- **Network SDK**: `sdk/go/net/` - HTTP, DNS, TCP, SMTP clients
