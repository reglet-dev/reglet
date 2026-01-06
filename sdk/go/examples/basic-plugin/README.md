# Basic HTTP Check Plugin

A complete example plugin demonstrating the Reglet Go SDK.

## Features

- ✅ HTTP endpoint availability check
- ✅ Status code validation
- ✅ Response body content check
- ✅ Safe config extraction (no panics)
- ✅ Proper error handling
- ✅ Structured logging

## Configuration

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `url` | string | ✅ | - | HTTP(S) URL to check |
| `expected_code` | int | ❌ | 200 | Expected HTTP status code |
| `check_contains` | string | ❌ | - | String that response should contain |

## Building

```bash
cd sdk/go/examples/basic-plugin
GOOS=wasip1 GOARCH=wasm go build -o basic-plugin.wasm main.go
```

## Example Profile

```yaml
name: http-check-example
description: Check if example.com is available

controls:
  - id: EXAMPLE-001
    name: Example.com Availability
    description: Verify example.com returns 200
    plugin: basic-http-check
    config:
      url: "https://example.com"
      expected_code: 200

  - id: EXAMPLE-002
    name: API Health Check
    description: Verify API health endpoint
    plugin: basic-http-check
    config:
      url: "https://api.example.com/health"
      expected_code: 200
      check_contains: "\"status\":\"ok\""
```

## Evidence Output

On success:
```json
{
  "status": true,
  "data": {
    "url": "https://example.com",
    "status_code": 200,
    "expected_code": 200,
    "code_matches": true,
    "content_matches": true
  }
}
```

On failure:
```json
{
  "status": false,
  "error": {
    "message": "check failed: status=503 (expected 200), content_match=false",
    "type": "internal"
  },
  "data": {
    "url": "https://example.com",
    "status_code": 503,
    "expected_code": 200,
    "code_matches": false
  }
}
```

## SDK Patterns Demonstrated

### Safe Config Extraction

```go
// Required field - returns error if missing
url, err := sdk.MustGetString(config, "url")

// Optional field with default
expectedCode := sdk.GetIntDefault(config, "expected_code", 200)
```

### HTTP Requests

```go
resp, err := sdknet.Get(ctx, url)
if err != nil {
    return sdk.Failure("network", err.Error()), nil
}
defer resp.Body.Close()
```

### Structured Logging

```go
slog.InfoContext(ctx, "Starting check", "url", url)
```

### Evidence Generation

```go
return sdk.Success(map[string]interface{}{
    "status_code": resp.StatusCode,
}), nil
```
