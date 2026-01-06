# HTTP Plugin

HTTP/HTTPS request checking and validation.

## Configuration

### Schema

```yaml
controls:
  - id: HTTP-001
    plugin: http
    config:
      url: "https://api.example.com/health"
      method: "GET"                          # Optional, default: "GET"
      body: ""                               # Optional: request body
      expected_status: 200                    # Optional: expected HTTP status
      expected_body_contains: "\"status\":\"ok\""  # Optional: substring to find
      body_preview_length: 200                # Optional: chars to include (0=hash only, -1=full)
```

### Required Fields

- `url`: URL to request (must include protocol).

### Optional Fields

- `method`: HTTP method (`GET`, `POST`, `PUT`, `DELETE`, `HEAD`, `OPTIONS`, `PATCH`). Default: `GET`.
- `body`: Request body for POST/PUT/PATCH requests.
- `expected_status`: Expected HTTP status code. If set, evidence includes `expectation_failed` field.
- `expected_body_contains`: String that response body should contain.
- `body_preview_length`: Number of characters to include from response (default: 200, 0=hash only, -1=full body).

## Capabilities

- **network**: `outbound:80,443`

## Evidence Data

### Success

```json
{
  "status": true,
  "data": {
    "status_code": 200,
    "response_time_ms": 150,
    "protocol": "HTTP/1.1",
    "headers": {
      "Content-Type": ["application/json"]
    },
    "body_size": 512,
    "body_sha256": "a1b2c3d4...",
    "body_preview": "{\"status\":\"ok\",\"uptime\":..."
  }
}
```

### Expectation Failed

```json
{
  "status": true,
  "data": {
    "status_code": 503,
    "response_time_ms": 150,
    "expectation_failed": true,
    "expectation_error": "expected status 200, got 503"
  }
}
```

### Network Failure

```json
{
  "status": false,
  "error": {
    "message": "network http_request failed for https://api.example.com: connection refused",
    "type": "network"
  }
}
```

## Development

### Building

```bash
make -C plugins/http build
```

### Testing

```bash
make -C plugins/http test
```

## Platform Requirements

- Reglet Host v0.2.0+
- WASM Runtime with `wasi_snapshot_preview1` support
