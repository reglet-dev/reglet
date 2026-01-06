# TCP Plugin

TCP connection testing and TLS validation.

## Configuration

### Schema

```yaml
controls:
  - id: TCP-001
    plugin: tcp
    config:
      host: "example.com"
      port: "443"
      timeout_ms: 5000              # Optional, default: 5000
      tls: true                     # Optional: use TLS/SSL
      expected_tls_version: "TLS 1.2"  # Optional: minimum TLS version
```

### Required Fields

- `host`: Target host (hostname or IP address).
- `port`: Target port.

### Optional Fields

- `timeout_ms`: Connection timeout in milliseconds (default: 5000).
- `tls`: Use TLS/SSL connection (default: false).
- `expected_tls_version`: Expected minimum TLS version (e.g., "TLS 1.2", "TLS 1.3").

## Capabilities

- **network**: `outbound:*`

## Evidence Data

### Success (Plain TCP)

```json
{
  "status": true,
  "data": {
    "connected": true,
    "address": "example.com:80",
    "response_time_ms": 45,
    "remote_addr": "93.184.216.34:80",
    "local_addr": "192.168.1.100:54321"
  }
}
```

### Success (TLS)

```json
{
  "status": true,
  "data": {
    "connected": true,
    "address": "example.com:443",
    "response_time_ms": 120,
    "tls": true,
    "tls_version": "TLS 1.3",
    "tls_cipher_suite": "TLS_AES_256_GCM_SHA384",
    "tls_server_name": "example.com",
    "tls_cert_subject": "CN=example.com",
    "tls_cert_issuer": "CN=Let's Encrypt Authority X3",
    "tls_cert_not_after": "2025-06-15T00:00:00Z",
    "tls_cert_days_remaining": 180
  }
}
```

### TLS Version Expectation Failed

```json
{
  "status": true,
  "data": {
    "connected": true,
    "tls": true,
    "tls_version": "TLS 1.1",
    "expectation_failed": true,
    "expectation_error": "expected TLS version >= TLS 1.2, got TLS 1.1"
  }
}
```

### Connection Failure

```json
{
  "status": false,
  "error": {
    "message": "network tcp_connect failed for example.com:443: connection refused",
    "type": "network"
  }
}
```

## Development

### Building

```bash
make -C plugins/tcp build
```

### Testing

```bash
make -C plugins/tcp test
```

## Platform Requirements

- Reglet Host v0.2.0+
- WASM Runtime with `wasi_snapshot_preview1` support
