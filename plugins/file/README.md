# File Plugin

The file plugin checks file existence, permissions, and content.

## Status

🚧 **Under Development** - This is a proof-of-concept plugin to validate the WASM runtime infrastructure.

## Current Implementation

This is a **placeholder** implementation that demonstrates the plugin structure. It does NOT yet implement proper WIT bindings.

### What Works
- Basic Go structure with exported functions
- Can be compiled to WASM

### What's Missing
- WIT bindings for proper data marshaling
- Memory management between host and WASM
- Proper Config/Evidence serialization
- Host function calls for filesystem access

## Building

To compile to WASM:

```bash
cd plugins/file
GOOS=wasip1 GOARCH=wasm go build -o file.wasm main.go
```

Or use the Makefile:

```bash
make -C plugins/file build
```

## Configuration

Once implemented, the plugin will accept this configuration:

```yaml
observations:
  - plugin: file
    config:
      path: /etc/ssh/sshd_config    # Required: Path to check
      mode: content                  # Optional: exists|readable|content
```

## Evidence

Will return:

```json
{
  "timestamp": "2025-01-15T10:30:00Z",
  "data": {
    "exists": true,
    "readable": true,
    "size_bytes": 3456,
    "modified": "2025-01-10T08:00:00Z",
    "content": "...file contents..."
  }
}
```

## Capabilities

Requires:
- `fs:read:<path pattern>`

Example grant in system config:

```yaml
plugins:
  reglet/file@1.0:
    capabilities:
      - fs:read:/etc/**
      - fs:read:/var/log/**
```

## Next Steps

1. Implement WIT bindings using wit-bindgen-go (or manual implementation)
2. Add proper memory management for string/data passing
3. Call host functions for filesystem access (respects capabilities)
4. Test end-to-end with Reglet runtime
5. Add comprehensive tests
