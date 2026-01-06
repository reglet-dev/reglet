# DNS Plugin

DNS resolution and record validation.

## Configuration

### Schema

```yaml
controls:
  - id: DNS-001
    plugin: dns
    config:
      hostname: "example.com"
      record_type: "A"              # Optional, default: "A"
      nameserver: "8.8.8.8:53"      # Optional, uses host's resolver if empty
```

### Required Fields

- `hostname`: The domain name to resolve (e.g., "example.com").

### Optional Fields

- `record_type`: The type of DNS record to query.
  - Values: `A`, `AAAA`, `CNAME`, `MX`, `TXT`, `NS`
  - Default: `A`
- `nameserver`: Custom nameserver to use for the query (e.g., "8.8.8.8:53").

## Capabilities

- **network**: `outbound:53`

## Evidence Data

### Success (A/AAAA/TXT/NS/CNAME)

```json
{
  "status": true,
  "data": {
    "hostname": "example.com",
    "record_type": "A",
    "records": ["93.184.216.34"],
    "record_count": 1,
    "query_time_ms": 45,
    "is_timeout": false,
    "is_not_found": false
  }
}
```

### Success (MX Records)

```json
{
  "status": true,
  "data": {
    "hostname": "example.com",
    "record_type": "MX",
    "mx_records": [
      {"host": "mail.example.com", "pref": 10},
      {"host": "mail2.example.com", "pref": 20}
    ],
    "record_count": 2,
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
    "type": "network"
  },
  "data": {
    "hostname": "nonexistent.example.com",
    "record_type": "A",
    "query_time_ms": 100,
    "is_timeout": false,
    "is_not_found": true
  }
}
```

## Development

### Building

```bash
make -C plugins/dns build
```

### Testing

```bash
make -C plugins/dns test
```

## Platform Requirements

- Reglet Host v0.2.0+
- WASM Runtime with `wasi_snapshot_preview1` support
