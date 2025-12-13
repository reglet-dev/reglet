# WASM Runtime Package

This package provides the WebAssembly runtime infrastructure for Reglet plugins using [wazero](https://github.com/tetratelabs/wazero).

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
  ├── Instance (running module)
  ├── Cached PluginInfo (from describe())
  └── Cached ConfigSchema (from schema())
```

## Key Types

### `Runtime`
Main runtime manager. Create one per Reglet execution:

```go
ctx := context.Background()
runtime, err := wasm.NewRuntime(ctx)
defer runtime.Close()
```

### `Plugin`
Wrapper around a WASM module. Provides methods to call WIT interface functions:

```go
// Load plugin
plugin, err := runtime.LoadPlugin("file", wasmBytes)

// Get metadata
info, err := plugin.Describe()

// Get config schema
schema, err := plugin.Schema()

// Execute observation
result, err := plugin.Observe(config)
```

### Type Mappings

Go types map to WIT interface types:

| Go Type | WIT Type |
|---------|----------|
| `PluginInfo` | `plugin-info` |
| `Capability` | `capability` |
| `Config` | `config` |
| `Evidence` | `evidence` |
| `PluginError` | `error` |
| `ConfigSchema` | `config-schema` |

## Current Status

**Phase 1a - Basic Infrastructure:**

✅ Runtime initialization with wazero
✅ Plugin loading and module compilation
✅ Type definitions matching WIT interface
✅ Basic tests

**TODO - WIT Bindings:**

The current implementation has placeholder TODOs for WIT bindings:

1. **Marshal/Unmarshal**: Need to implement proper data serialization between Go and WASM memory
2. **Host Functions**: Need to implement capability-enforced host functions
3. **Memory Management**: Need to handle WASM linear memory for passing complex data structures

This will be completed after we build a simple plugin to validate the approach.

## Host Functions (Planned)

Host functions will provide sandboxed access to system resources:

### Filesystem
- `fs_read(path: string) -> result<bytes, error>`
- `fs_write(path: string, data: bytes) -> result<void, error>`
- Enforces `fs:read:<glob>` and `fs:write:<glob>` capabilities

### Network
- `net_connect(host: string, port: u16) -> result<connection, error>`
- Enforces `network:outbound:<ports>` capability

### Environment
- `env_get(name: string) -> result<string, error>`
- Enforces `env:<pattern>` capability

### Execution
- `exec_run(command: string, args: []string) -> result<output, error>`
- Enforces `exec:<commands>` capability

## Security Model

### Capability Enforcement

1. Plugin declares required capabilities in `describe()`
2. System config grants capabilities to plugins
3. Runtime checks capabilities on every host function call
4. Unauthorized access is denied with clear error

### Sandboxing

- WASM provides memory isolation (plugins can't access host memory directly)
- wazero is pure Go (no CGO, no OS syscalls from plugins)
- All system access goes through capability-checked host functions
- Timeouts prevent infinite loops

## Testing

Run tests:
```bash
make test
```

Current test coverage:
- Runtime initialization
- Plugin loading with invalid WASM
- Plugin cache/retrieval

TODO: Add tests with actual WASM plugins once we build the file plugin.

## Dependencies

- `github.com/tetratelabs/wazero` - Pure Go WASM runtime
- `github.com/stretchr/testify` - Testing framework

## Next Steps

1. Build a simple file plugin in Go that compiles to WASM
2. Implement WIT bindings for data marshaling
3. Test end-to-end: load plugin, call describe/observe
4. Implement host functions with capability checking
5. Add comprehensive tests with real plugins
