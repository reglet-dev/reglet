# Reglet Go SDK

The Reglet Go SDK provides Go APIs for writing WebAssembly (WASM) plugins for the Reglet compliance platform. It handles memory management, host communication, and plugin registration.

## Installation

The SDK is intended to be used within the Reglet plugin development environment.

```bash
go get github.com/whiskeyjimbo/reglet/sdk
```

## Usage

### Plugin Registration

Plugins must implement the `sdk.Plugin` interface and register themselves in `init()`.

```go
package main

import (
    "context"
    "log/slog"
    regletsdk "github.com/whiskeyjimbo/reglet/sdk"
)

type myPlugin struct{}

func init() {
    slog.Info("Registering my plugin")
    regletsdk.Register(&myPlugin{})
}

// ... implement Describe, Schema, and Check methods ...
```

### Network Operations

The SDK provides wrappers for network operations to ensure they work correctly within the WASM sandbox.

#### DNS

Due to limitations in intercepting the standard library's DNS resolver in WASM, plugins **must** use the SDK's `net` package for DNS lookups instead of `net.LookupHost`.

```go
import (
    "context"
    regletnet "github.com/whiskeyjimbo/reglet/sdk/net"
)

func checkDNS(ctx context.Context, hostname string) error {
    // CORRECT: Use SDK wrapper
    addrs, err := regletnet.LookupHost(ctx, hostname)
    if err != nil {
        return err
    }
    
    // ...
    return nil
}
```

**Note:** `net.LookupHost` (standard library) will not work as expected and may fail or attempt prohibited network calls.

#### HTTP

The SDK automatically intercepts `http.DefaultTransport`, so standard `http.Get`, `http.Post`, etc., work out of the box.

```go
import "net/http"

func checkHTTP(url string) error {
    resp, err := http.Get(url) // Works correctly via SDK shim
    // ...
}
```

### Logging

Use `log/slog` for logging. The SDK configures the default logger to send structured logs to the host.

```go
import "log/slog"

func myFunc() {
    slog.Info("Something happened", "key", "value")
}
```

## Limitations

*   **DNS**: `net.LookupHost` is not supported. Use `regletnet.LookupHost`.
*   **Concurrency**: Plugins are single-threaded (WASI limitation). Use `errgroup` for logical concurrency, but execution is serial.
*   **Filesystem**: Access is restricted to the sandbox. Use the capabilities system to request access.
