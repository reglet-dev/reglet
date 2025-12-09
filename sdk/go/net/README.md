# Net Package

The `net` package provides network operations for Reglet WASM plugins, including DNS resolution, HTTP requests, and TCP connections. All network operations are routed through the host via WASM host functions.

## Overview

This package wraps the host's network functionality, allowing plugins to perform network operations without direct access to the host's network stack. All operations require explicit capability grants and are sandboxed.

## Security Model

- **Requires Capabilities**:
  - `network:outbound` - General network access
  - `network:outbound:<host>:<port>` - Specific host/port access
  - `network:dns` - DNS resolution
- **Sandboxed**: No direct network stack access
- **Host-Controlled**: Host enforces rate limits, timeouts, and allowed destinations

## Table of Contents

- [DNS Resolution](#dns-resolution)
- [HTTP Requests](#http-requests)
- [TCP Connections](#tcp-connections)
- [Wire Format](#wire-format)
- [Context Propagation](#context-propagation)
- [Common Patterns](#common-patterns)

---

## DNS Resolution

### Basic Usage

```go
package main

import (
    "context"
    "log/slog"

    "github.com/whiskeyjimbo/reglet/sdk"
    "github.com/whiskeyjimbo/reglet/sdk/net"
)

type DNSPlugin struct{}

func (p *DNSPlugin) Check(ctx context.Context, config sdk.Config) (sdk.Evidence, error) {
    // Create DNS resolver
    resolver := &net.WasmResolver{
        Nameserver: "", // Empty = use host's default resolver
    }

    // Resolve IP addresses
    ips, err := resolver.LookupHost(ctx, "example.com")
    if err != nil {
        return sdk.Failure("dns", err.Error()), nil
    }

    return sdk.Success(map[string]interface{}{
        "hostname": "example.com",
        "ips":      ips,
    }), nil
}
```

### WasmResolver API

```go
type WasmResolver struct {
    Nameserver string // Optional: custom nameserver (e.g., "8.8.8.8:53")
}
```

#### LookupHost

Resolve hostname to IP addresses (both A and AAAA records):

```go
resolver := &net.WasmResolver{}
ips, err := resolver.LookupHost(ctx, "example.com")
// Returns: ["93.184.216.34", "2606:2800:220:1:248:1893:25c8:1946"]
```

#### LookupIPAddr

Resolve hostname to structured IP addresses:

```go
ipAddrs, err := resolver.LookupIPAddr(ctx, "example.com")
for _, addr := range ipAddrs {
    fmt.Println(addr.IP)  // net.IP type
}
```

#### LookupCNAME

Get canonical name for a hostname:

```go
cname, err := resolver.LookupCNAME(ctx, "www.example.com")
// Returns: "example.com."
```

#### LookupMX

Get MX records as formatted strings:

```go
mxRecords, err := resolver.LookupMX(ctx, "example.com")
// Returns: ["10 mail.example.com", "20 mail2.example.com"]
```

#### LookupMXRecords

Get structured MX records:

```go
mxRecords, err := resolver.LookupMXRecords(ctx, "example.com")
for _, mx := range mxRecords {
    fmt.Printf("Priority: %d, Host: %s\n", mx.Pref, mx.Host)
}
```

#### LookupTXT

Get TXT records:

```go
txtRecords, err := resolver.LookupTXT(ctx, "example.com")
// Returns: ["v=spf1 include:_spf.google.com ~all"]
```

#### LookupNS

Get nameserver records:

```go
nsRecords, err := resolver.LookupNS(ctx, "example.com")
// Returns: ["ns1.example.com.", "ns2.example.com."]
```

### Custom Nameserver

```go
resolver := &net.WasmResolver{
    Nameserver: "1.1.1.1:53", // Use Cloudflare DNS
}

ips, err := resolver.LookupHost(ctx, "example.com")
```

### DNS Best Practices

1. **Use Timeouts**: Always use context with timeout for DNS queries
2. **Handle Failures**: DNS lookups can fail; implement retry logic if needed
3. **Cache Results**: Consider caching DNS results to reduce queries
4. **NXDOMAIN**: Handle "no such host" errors gracefully

---

## HTTP Requests

### ⚠️ Breaking Change (v0.1.0-alpha)

HTTP transport is now **explicit** instead of implicit. You must either:
1. Use SDK helper functions (recommended)
2. Create an `http.Client` with `WasmTransport`

### Basic Usage (Recommended)

```go
import (
    "context"
    "io"

    sdknet "github.com/whiskeyjimbo/reglet/sdk/net"
)

func (p *HTTPPlugin) Check(ctx context.Context, config sdk.Config) (sdk.Evidence, error) {
    // Simple GET request
    resp, err := sdknet.Get(ctx, "https://api.example.com/status")
    if err != nil {
        return sdk.Failure("http", err.Error()), nil
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return sdk.Failure("http", err.Error()), nil
    }

    return sdk.Success(map[string]interface{}{
        "status_code": resp.StatusCode,
        "body":        string(body),
    }), nil
}
```

### HTTP Helper Functions

#### Get

```go
func Get(ctx context.Context, url string) (*http.Response, error)
```

Convenience function for GET requests:

```go
resp, err := sdknet.Get(ctx, "https://httpbin.org/get")
```

#### Post

```go
func Post(ctx context.Context, url, contentType string, body io.Reader) (*http.Response, error)
```

Convenience function for POST requests:

```go
import "bytes"

body := bytes.NewReader([]byte(`{"key":"value"}`))
resp, err := sdknet.Post(ctx, "https://httpbin.org/post", "application/json", body)
```

#### Do

```go
func Do(req *http.Request) (*http.Response, error)
```

Execute a custom HTTP request:

```go
req, _ := http.NewRequestWithContext(ctx, "PUT", url, body)
req.Header.Set("Authorization", "Bearer "+token)
req.Header.Set("Content-Type", "application/json")

resp, err := sdknet.Do(req)
```

### Advanced HTTP Usage

#### Custom Client

```go
client := &http.Client{
    Transport: &net.WasmTransport{},
    Timeout:   10 * time.Second,
}

resp, err := client.Get("https://example.com")
```

#### Request Headers

```go
req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
req.Header.Set("User-Agent", "RegletPlugin/1.0")
req.Header.Set("Accept", "application/json")

resp, err := sdknet.Do(req)
```

#### JSON Requests

```go
import "encoding/json"

data := map[string]interface{}{
    "username": "user",
    "action":   "login",
}

jsonData, _ := json.Marshal(data)
body := bytes.NewReader(jsonData)

resp, err := sdknet.Post(ctx, url, "application/json", body)
```

### HTTP Response Body Size Limit

**Important**: HTTP response bodies are limited to **10 MB** (`MaxHTTPBodySize`).

If a response exceeds this limit, you'll receive an error:

```go
resp, err := sdknet.Get(ctx, url)
if err != nil {
    // err: "HTTP response body exceeds maximum size (10485760 bytes)"
    return sdk.Failure("http", "Response too large"), nil
}
```

### HTTP Best Practices

1. **Always Close Response Bodies**: `defer resp.Body.Close()`
2. **Check Status Codes**: Don't assume 2xx responses
3. **Set Timeouts**: Use context with timeout for all requests
4. **Handle Redirects**: Client follows redirects by default
5. **Check Content-Length**: Verify response size before reading

---

## TCP Connections

### Basic Usage

```go
import (
    "context"

    "github.com/whiskeyjimbo/reglet/sdk"
    "github.com/whiskeyjimbo/reglet/sdk/net"
)

func (p *TCPPlugin) Check(ctx context.Context, config sdk.Config) (sdk.Evidence, error) {
    // Connect to TCP service
    conn, err := net.DialTCP(ctx, "tcp", "example.com:443")
    if err != nil {
        return sdk.Failure("tcp", err.Error()), nil
    }

    return sdk.Success(map[string]interface{}{
        "host":      "example.com:443",
        "connected": true,
        "duration":  conn.Duration,
    }), nil
}
```

### DialTCP API

```go
func DialTCP(ctx context.Context, network, address string) (*TCPConnection, error)
```

**Parameters:**
- `network`: "tcp", "tcp4", or "tcp6"
- `address`: "host:port" format

**Returns:**
- `TCPConnection`: Connection metadata
- `error`: Connection error if failed

### TCPConnection

```go
type TCPConnection struct {
    RemoteAddr string        // Remote address (host:port)
    LocalAddr  string        // Local address
    Duration   time.Duration // Connection establishment time
    TLS        bool          // Whether TLS is enabled
    TLSVersion string        // TLS version (if TLS enabled)
}
```

### TLS Connections

```go
// TLS connection (requires TLS on target port)
conn, err := net.DialTCP(ctx, "tcp", "example.com:443")
if err != nil {
    return sdk.Failure("tcp", err.Error()), nil
}

return sdk.Success(map[string]interface{}{
    "tls_enabled": conn.TLS,
    "tls_version": conn.TLSVersion,
}), nil
```

### TCP Best Practices

1. **Use Timeouts**: Always use context with timeout
2. **Handle Connection Failures**: Network issues are common
3. **Check Ports**: Verify target ports are reachable
4. **TLS Validation**: Check TLS status for secure connections

---

## Wire Format

All network operations use JSON-based wire format:

### DNS Request/Response

```json
// Request
{
    "context": { /* context metadata */ },
    "hostname": "example.com",
    "type": "A",
    "nameserver": ""
}

// Response
{
    "records": ["93.184.216.34"],
    "mx_records": [],
    "error": null
}
```

### HTTP Request/Response

```json
// Request
{
    "context": { /* context metadata */ },
    "method": "GET",
    "url": "https://example.com",
    "headers": {"User-Agent": "Reglet/1.0"},
    "body": ""
}

// Response
{
    "status_code": 200,
    "headers": {"Content-Type": "text/html"},
    "body": "base64-encoded-body",
    "body_truncated": false,
    "error": null
}
```

### TCP Request/Response

```json
// Request
{
    "context": { /* context metadata */ },
    "network": "tcp",
    "address": "example.com:443",
    "timeout_ms": 5000
}

// Response
{
    "remote_addr": "93.184.216.34:443",
    "local_addr": "192.168.1.100:54321",
    "duration": "150ms",
    "tls": true,
    "tls_version": "TLS 1.3",
    "error": null
}
```

---

## Context Propagation

All network functions fully support Go context propagation:

### Cancellation

```go
ctx, cancel := context.WithCancel(context.Background())

go func() {
    time.Sleep(100 * time.Millisecond)
    cancel()  // Cancel the request
}()

resp, err := sdknet.Get(ctx, "https://example.com")
// err: context canceled
```

### Timeouts

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

resp, err := sdknet.Get(ctx, "https://slow-api.example.com")
// err: context deadline exceeded (if > 5 seconds)
```

### Values

```go
ctx := context.WithValue(context.Background(), "request_id", "abc123")

// request_id is passed to host for logging/tracing
resp, err := sdknet.Get(ctx, url)
```

---

## Common Patterns

### Check DNS and HTTP

```go
func (p *MyPlugin) Check(ctx context.Context, config sdk.Config) (sdk.Evidence, error) {
    host := "example.com"

    // 1. DNS resolution
    resolver := &net.WasmResolver{}
    ips, err := resolver.LookupHost(ctx, host)
    if err != nil {
        return sdk.Failure("dns", err.Error()), nil
    }

    // 2. HTTP request
    resp, err := sdknet.Get(ctx, "https://"+host)
    if err != nil {
        return sdk.Failure("http", err.Error()), nil
    }
    defer resp.Body.Close()

    return sdk.Success(map[string]interface{}{
        "dns_ips":     ips,
        "http_status": resp.StatusCode,
    }), nil
}
```

### Retry with Backoff

```go
func fetchWithRetry(ctx context.Context, url string, maxRetries int) (*http.Response, error) {
    for i := 0; i < maxRetries; i++ {
        resp, err := sdknet.Get(ctx, url)
        if err == nil {
            return resp, nil
        }

        if i < maxRetries-1 {
            backoff := time.Duration(i+1) * time.Second
            time.Sleep(backoff)
        }
    }
    return nil, fmt.Errorf("max retries exceeded")
}
```

### Health Check

```go
func checkHealth(ctx context.Context, endpoint string) bool {
    resp, err := sdknet.Get(ctx, endpoint)
    if err != nil {
        return false
    }
    defer resp.Body.Close()

    return resp.StatusCode >= 200 && resp.StatusCode < 300
}
```

---

## Limitations

1. **Response Size**: HTTP bodies limited to 10 MB
2. **No Streaming**: HTTP responses are fully buffered
3. **No WebSockets**: Not supported
4. **TCP Read/Write**: TCP connections are check-only, no bidirectional communication
5. **No UDP**: UDP protocol not supported
6. **No Raw Sockets**: Only standard protocols (HTTP, TCP, DNS)

---

## Migration Guide (v0.1.0-alpha)

### Breaking Change 1: DNS Functions Removed

**Before:**
```go
ips, err := net.LookupHost(ctx, "example.com")
```

**After:**
```go
resolver := &net.WasmResolver{}
ips, err := resolver.LookupHost(ctx, "example.com")
```

### Breaking Change 2: HTTP Transport

**Before:**
```go
// Implicit transport (no longer works)
resp, err := http.Get("https://example.com")
```

**After (Option 1 - Recommended):**
```go
import sdknet "github.com/whiskeyjimbo/reglet/sdk/net"
resp, err := sdknet.Get(ctx, "https://example.com")
```

**After (Option 2 - Custom Client):**
```go
client := &http.Client{Transport: &net.WasmTransport{}}
resp, err := client.Get("https://example.com")
```

---

## See Also

- [Main SDK Documentation](../README.md)
- [Context Package](../internal/context/README.md)
- [Plugin Development Guide](../../docs/plugin-development.md)
