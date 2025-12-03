# dns Plugin

DNS resolution and record validation.

## Configuration

### Schema

```yaml
observations:
  - plugin: dns
    config:
      hostname: "example.com"
      record_type: "A" # Optional, default: "A"
      nameserver: "8.8.8.8:53" # Optional (ignored in current version)
```

### Required Fields

-   `hostname`: The domain name to resolve (e.g., "example.com").

### Optional Fields

-   `record_type`: The type of DNS record to query.
    -   Values: `A`, `AAAA`, `CNAME`, `MX`, `TXT`, `NS`
    -   Default: `A`
-   `nameserver`: Custom nameserver to use for the query.
    -   *Note: Currently ignored by the runtime, which uses the host's resolver.*

## Capabilities

-   **network**: `outbound:53`

## Evidence Data

The plugin returns the following evidence structure:

### Success

```json
{
  "status": true,
  "data": {
    "hostname": "example.com",
    "record_type": "A",
    "records": ["93.184.216.34"],
    "record_count": 1,
    "query_time_ms": 45
  }
}
```

### Failure

```json
{
  "status": false,
  "error": {
    "message": "DNS lookup failed: ...",
    "type": "network",
    "wrapped": { ... }
  }
}
```

## Development

### Building

```bash
make build
```

### Testing

```bash
make test
```

## Platform Requirements

-   Reglet Host v0.2.0+
-   WASM Runtime with `wasi_snapshot_preview1` support
