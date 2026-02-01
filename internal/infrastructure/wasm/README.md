# WASM Runtime Package

This package provides the WebAssembly runtime infrastructure for Reglet plugins using [wazero](https://github.com/tetratelabs/wazero).

## WASM Compilation

**IMPORTANT: We use standard Go (1.21+), NOT TinyGo.**

Plugins are compiled to WASM using:

```bash
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o plugin.wasm .
```

TinyGo has limited reflection support which breaks the SDK's service registration pattern.

## Overview

All Reglet plugins are WASM modules (including embedded ones). This package:

- Loads and executes WASM plugins
- Enforces capability-based sandboxing
- Provides host functions for filesystem, network, and system access
- Manages plugin lifecycle and resource cleanup

## Architecture

```
Runtime
  ├── wazero.Runtime (WASM engine)
  ├── CapabilityManager (security enforcement)
  └── Map<name, Plugin> (loaded plugins)

Plugin
  ├── CompiledModule (WASM bytecode)
  ├── Instance pool (pre-warmed instances)
  ├── Cached Manifest (from Manifest())
  └── GrantSet (capabilities converted from Manifest)
```

## Key Types

### `Runtime`
Main runtime manager. Create one per Reglet execution:

```go
ctx := context.Background()
runtime, err := wasm.NewRuntime(ctx, buildInfo)
defer runtime.Close(ctx)
```

### `Plugin`
Wrapper around a WASM module. Provides methods to call plugin interface functions:

```go
// Load plugin
plugin, err := runtime.LoadPlugin(ctx, "file", wasmBytes)

// Get manifest (includes name, version, capabilities, config schema, services)
manifest, err := plugin.Manifest(ctx)

// Execute observation
result, err := plugin.Observe(ctx, config)
```

### Type Mappings

Go types map to SDK entity types:

| Go Type | SDK Type |
|---------|----------|
| `entities.Manifest` | Plugin metadata (name, version, capabilities, schema, services) |
| `entities.Capability` | Single capability declaration |
| `entities.GrantSet` | Structured capability set for enforcement |
| `Config` | Plugin configuration |
| `Evidence` | Observation result with status and data |
| `PluginError` | Error details from plugin |

## Current Status

**Complete:**

✅ Runtime initialization with wazero
✅ Plugin loading and module compilation
✅ `Manifest()` method for plugin metadata
✅ `Observe()` method for plugin execution
✅ Capability-to-GrantSet conversion
✅ Instance pooling for concurrent execution
✅ Host functions with capability enforcement
✅ Comprehensive tests including race detection

## Host Functions

Host functions provide sandboxed access to system resources (see `hostfuncs/`):

| Function | Capability | Description |
|----------|------------|-------------|
| HTTP | `network:outbound` | HTTP client with TLS |
| DNS | `network:outbound` | DNS resolution |
| TCP | `network:outbound` | Raw TCP connections |
| SMTP | `network:outbound` | SMTP client |
| Exec | `exec:<command>` | Command execution |
| Filesystem | `fs:read`, `fs:write` | File access via WASI |
| Environment | `env:<pattern>` | Environment variable access |

## Security Model

### Capability Enforcement

1. Plugin declares required capabilities in `Manifest().Capabilities`
2. System config grants capabilities to plugins
3. Runtime converts capabilities to `GrantSet` for enforcement
4. Host functions check capabilities on every call
5. Unauthorized access is denied with clear error

### Sandboxing

- WASM provides memory isolation (plugins can't access host memory directly)
- wazero is pure Go (no CGO, no OS syscalls from plugins)
- All system access goes through capability-checked host functions
- Timeouts prevent infinite loops

## Testing

Run tests:
```bash
go test ./internal/infrastructure/wasm/... -v
```

Test coverage includes:
- Runtime initialization and configuration
- Plugin loading and caching
- `Manifest()` and `Observe()` integration tests
- Concurrent execution with race detection
- Host function capability enforcement
- Fuzz testing for wire format parsing

## Dependencies

- `github.com/tetratelabs/wazero` - Pure Go WASM runtime (no CGO)
- `github.com/reglet-dev/reglet-sdk` - Plugin SDK entities
- `github.com/stretchr/testify` - Testing framework

## Plugin Interface

Plugins must implement two exported functions:

```go
// Returns plugin manifest as JSON (packed ptr|len)
//go:wasmexport manifest
func _manifest() uint64

// Executes observation with config, returns result as JSON (packed ptr|len)
//go:wasmexport observe
func _observe(configPtr uint32, configLen uint32) uint64
```

The SDK handles these exports automatically when using `plugin.Register()`.
