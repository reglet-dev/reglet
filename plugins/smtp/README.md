# SMTP Plugin

SMTP connection testing and server validation.

## Configuration

### Schema

```yaml
controls:
  - id: SMTP-001
    plugin: smtp
    config:
      host: "mail.example.com"
      port: "587"
      timeout_ms: 5000              # Optional, default: 5000
      tls: false                    # Optional: direct TLS (port 465)
      starttls: true                # Optional: upgrade via STARTTLS (port 587)
```

### Required Fields

- `host`: SMTP server host (hostname or IP).
- `port`: SMTP server port (25, 465, 587, 2525).

### Optional Fields

- `timeout_ms`: Connection timeout in milliseconds (default: 5000).
- `tls`: Use direct TLS/SSL connection - SMTPS (default: false, typical for port 465).
- `starttls`: Upgrade connection via STARTTLS (default: false, typical for port 587).

## Common Port Configurations

| Port | Protocol | tls | starttls |
|------|----------|-----|----------|
| 25   | Plain SMTP | `false` | `false` |
| 465  | SMTPS (implicit TLS) | `true` | `false` |
| 587  | Submission (STARTTLS) | `false` | `true` |

## Capabilities

- **network**: `outbound:25,465,587`

## Evidence Data

### Success

```json
{
  "status": true,
  "data": {
    "connected": true,
    "address": "mail.example.com:587",
    "response_time_ms": 85,
    "banner": "220 mail.example.com ESMTP Postfix",
    "tls": true,
    "tls_version": "TLS 1.3",
    "tls_cipher_suite": "TLS_AES_256_GCM_SHA384",
    "tls_server_name": "mail.example.com"
  }
}
```

### Connection Failure

```json
{
  "status": false,
  "error": {
    "message": "network smtp_connect failed for mail.example.com:587: connection refused",
    "type": "network"
  }
}
```

## Development

### Building

```bash
make -C plugins/smtp build
```

### Testing

```bash
make -C plugins/smtp test
```

## Platform Requirements

- Reglet Host v0.2.0+
- WASM Runtime with `wasi_snapshot_preview1` support
