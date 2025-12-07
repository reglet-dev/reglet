# File Plugin

The file plugin checks file existence, permissions, and content.

## Status

✅ **Operational** - Functional file checking implementation.

## Implementation

Uses standard Go `os` and `io` packages, compiled to WASI. The host (Reglet) mounts the root filesystem `/` to the WASI guest's `/`, allowing absolute path access.

### Features
- Check existence (`mode: exists`)
- Check readability (`mode: readable`)
- Read content (`mode: content`)
- Retrieve metadata (permissions, size, mod time)

## Building

To compile to WASM:

```bash
make -C plugins/file build
```

## Configuration

```yaml
observations:
  - plugin: file
    config:
      path: /etc/ssh/sshd_config    # Required: Path to check
      mode: exists                   # Optional: exists|readable|content (default: exists)
```

## Evidence

### `mode: exists`

Returns metadata about the file:

```json
{
  "exists": true,
  "is_dir": false,
  "size": 3456,
  "mode": "0644",
  "permissions": "-rw-r--r--",
  "mod_time": "2025-01-15T10:30:00Z"
}
```

### `mode: content`

Returns base64-encoded content:

```json
{
  "content_b64": "...",
  "encoding": "base64",
  "size": 3456
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