# File Plugin

File existence, content, and hash checks.

## Configuration

### Schema

```yaml
controls:
  - id: FILE-001
    plugin: file
    config:
      path: "/etc/ssh/sshd_config"     # Required: Path to check
      read_content: false               # Optional: Read and return file content (base64)
      hash: false                       # Optional: Calculate SHA256 hash
```

### Required Fields

- `path`: Absolute path to file to check.

### Optional Fields

- `read_content`: Read and return file content as base64 (default: `false`).
- `hash`: Calculate and return SHA256 hash of file (default: `false`).

## Capabilities

- **fs**: `read:**`

Example grant in system config:

```yaml
plugins:
  reglet/file@1.0:
    capabilities:
      - fs:read:/etc/**
      - fs:read:/var/log/**
```

## Evidence Data

### Basic Check (existence and metadata)

```json
{
  "status": true,
  "data": {
    "path": "/etc/ssh/sshd_config",
    "exists": true,
    "readable": true,
    "is_dir": false,
    "is_symlink": false,
    "size": 3456,
    "mode": "0644",
    "permissions": "-rw-r--r--",
    "mod_time": "2025-01-15T10:30:00Z",
    "uid": 0,
    "gid": 0
  }
}
```

### With `read_content: true`

```json
{
  "status": true,
  "data": {
    "path": "/etc/ssh/sshd_config",
    "exists": true,
    "readable": true,
    "content_b64": "IyBTU0hEIGNvbmZpZ3VyYXRpb24uLi4=",
    "encoding": "base64",
    "size": 3456
  }
}
```

### With `hash: true`

```json
{
  "status": true,
  "data": {
    "path": "/etc/ssh/sshd_config",
    "exists": true,
    "sha256": "a1b2c3d4e5f6..."
  }
}
```

### File Not Found

```json
{
  "status": true,
  "data": {
    "path": "/nonexistent/file",
    "exists": false,
    "readable": false
  }
}
```

## Development

### Building

```bash
make -C plugins/file build
```

### Testing

```bash
make -C plugins/file test
```

## Platform Requirements

- Reglet Host v0.2.0+
- WASM Runtime with `wasi_snapshot_preview1` support