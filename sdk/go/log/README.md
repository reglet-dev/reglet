# Log Package

The `log` package provides structured logging capabilities for Reglet WASM plugins using Go's standard `log/slog` package. It automatically routes all plugin logs to the host for centralized logging and observability.

## Overview

This package initializes a custom `slog.Handler` that sends log records from the WASM plugin to the host through a dedicated host function. Plugin authors can use standard Go logging (`slog`) without needing to understand WASM boundaries.

## Features

- **Standard Library Integration**: Uses Go's `log/slog` package
- **Structured Logging**: Full support for key-value attributes
- **Multiple Log Levels**: Debug, Info, Warn, Error
- **Context Aware**: Propagates context metadata to host
- **Zero Configuration**: Automatically initialized via `init()`

## Basic Usage

```go
package main

import (
    "context"
    "log/slog"

    "github.com/reglet-dev/reglet/sdk"
    _ "github.com/reglet-dev/reglet/sdk/log" // Import to initialize
)

type MyPlugin struct{}

func (p *MyPlugin) Check(ctx context.Context, config sdk.Config) (sdk.Evidence, error) {
    // Simple logging
    slog.Info("Starting check")
    slog.Debug("Config received", "keys", len(config))

    // Warning and errors
    if someCondition {
        slog.Warn("Unexpected condition detected", "value", someValue)
    }

    if err != nil {
        slog.Error("Operation failed", "error", err)
        return sdk.Failure("error", err.Error()), nil
    }

    slog.Info("Check completed successfully")
    return sdk.Success(map[string]interface{}{"status": "ok"}), nil
}
```

## Context-Aware Logging

All logging functions support context-aware variants:

```go
func (p *MyPlugin) Check(ctx context.Context, config sdk.Config) (sdk.Evidence, error) {
    // Context is propagated to host
    slog.InfoContext(ctx, "Starting check with context")

    // Context values (like request ID) are included
    slog.DebugContext(ctx, "Processing config", "config_size", len(config))

    // Errors with context
    if err != nil {
        slog.ErrorContext(ctx, "Failed to process", "error", err)
    }

    return sdk.Success(nil), nil
}
```

## Structured Attributes

Add structured key-value attributes to logs:

```go
// Simple attributes
slog.Info("User action", "user_id", 123, "action", "login")

// Multiple attributes
slog.Debug("API call",
    "method", "GET",
    "url", "/api/status",
    "duration_ms", 45,
)

// Error with context
slog.Error("Database error",
    "query", "SELECT * FROM users",
    "error", err,
    "retry_count", 3,
)
```

## Log Levels

### Debug

Verbose information for debugging:

```go
slog.Debug("Cache hit", "key", cacheKey, "ttl", ttl)
```

### Info

General informational messages:

```go
slog.Info("Plugin initialized", "version", "1.0.0")
```

### Warn

Warning conditions that should be investigated:

```go
slog.Warn("Slow operation", "duration_ms", 5000, "threshold_ms", 1000)
```

### Error

Error conditions requiring attention:

```go
slog.Error("Failed to connect", "host", "api.example.com", "error", err)
```

## Advanced Usage

### Log Groups

Group related attributes:

```go
slog.Info("HTTP request",
    slog.Group("request",
        slog.String("method", "POST"),
        slog.String("path", "/api/users"),
        slog.Int("status", 201),
    ),
    slog.Group("timing",
        slog.Duration("total", totalTime),
        slog.Duration("db", dbTime),
    ),
)
```

### Custom Attributes

```go
// Using slog.Attr for custom types
slog.Info("Operation complete",
    slog.String("operation", "backup"),
    slog.Time("completed_at", time.Now()),
    slog.Bool("success", true),
    slog.Any("metadata", customStruct),
)
```

### Conditional Logging

```go
// Only log if debug enabled
if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
    expensiveDebugInfo := computeDebugInfo()
    slog.Debug("Debug info", "data", expensiveDebugInfo)
}
```

## Log Handler Details

The package provides a `WasmLogHandler` that implements `slog.Handler`:

```go
type WasmLogHandler struct {
    // Configured as default handler via init()
}
```

### Wire Format

Logs are serialized to JSON and sent to the host:

```json
{
    "context": { /* context metadata */ },
    "level": "INFO",
    "message": "Operation completed",
    "timestamp": "2024-01-15T10:30:00Z",
    "attributes": {
        "duration_ms": 150,
        "user_id": 123
    },
    "source": {
        "file": "plugin.go",
        "line": 42,
        "function": "MyPlugin.Check"
    }
}
```

## Initialization

The log package automatically initializes itself when imported:

```go
import (
    _ "github.com/reglet-dev/reglet/sdk/log" // Initialize WASM logging
)
```

This sets up:
1. Custom `WasmLogHandler` as the default slog handler
2. Log level from environment or defaults to Info
3. Wire format protocol for host communication

## Best Practices

### 1. Use Appropriate Log Levels

```go
// ✅ Good
slog.Debug("Cache lookup", "key", key)           // Development info
slog.Info("Plugin started", "version", version)  // Important events
slog.Warn("Retry attempt", "count", retryCount)  // Potential issues
slog.Error("Failed to save", "error", err)       // Actual errors

// ❌ Bad
slog.Info("Variable x = 42")                     // Too verbose for Info
slog.Error("User not found")                     // Expected condition, not an error
```

### 2. Add Context with Structured Attributes

```go
// ✅ Good
slog.Info("User logged in", "user_id", userID, "ip", ipAddr)

// ❌ Bad
slog.Info(fmt.Sprintf("User %d logged in from %s", userID, ipAddr))
```

### 3. Use Context-Aware Functions

```go
// ✅ Good
slog.InfoContext(ctx, "Processing request", "request_id", reqID)

// ❌ Less useful
slog.Info("Processing request", "request_id", reqID)
```

### 4. Log Errors with Context

```go
// ✅ Good
slog.Error("Database query failed",
    "query", query,
    "error", err,
    "retry_count", retries,
)

// ❌ Bad
slog.Error(err.Error())
```

### 5. Avoid Sensitive Data

```go
// ✅ Good
slog.Info("Authentication successful", "user_id", userID)

// ❌ Bad - Leaks credentials
slog.Debug("Auth request", "password", password, "token", token)
```

## Performance Considerations

1. **Lazy Evaluation**: Expensive log message construction is only performed if the log level is enabled

2. **Buffering**: Log messages are buffered and sent to host asynchronously (host-side implementation)

3. **Attribute Limits**: Avoid extremely large attribute values (host may truncate)

4. **Debug Logs**: Disable debug logging in production for better performance

## Common Patterns

### Plugin Lifecycle Logging

```go
func (p *MyPlugin) Check(ctx context.Context, config sdk.Config) (sdk.Evidence, error) {
    slog.InfoContext(ctx, "Check started", "plugin", p.Name())

    defer func() {
        slog.InfoContext(ctx, "Check completed", "plugin", p.Name())
    }()

    // ... plugin logic
}
```

### Operation Timing

```go
start := time.Now()
result, err := doExpensiveOperation()
duration := time.Since(start)

if duration > 5*time.Second {
    slog.Warn("Slow operation detected",
        "operation", "expensive_op",
        "duration_ms", duration.Milliseconds(),
    )
}
```

### Error Recovery

```go
defer func() {
    if r := recover(); r != nil {
        slog.Error("Plugin panic recovered",
            "panic", r,
            "stack", debug.Stack(),
        )
    }
}()
```

## Limitations

1. **No Direct File Output**: Logs always go to host, cannot write to files directly
2. **Log Level Control**: Log level is controlled by host, not configurable in plugin
3. **Output Format**: Output formatting (JSON, text, etc.) determined by host
4. **Performance**: Cross-WASM-boundary logging has overhead compared to native logging

## Comparison with Standard Logging

| Feature | WASM Plugin | Native Go |
|---------|-------------|-----------|
| slog API | ✅ Full support | ✅ Native |
| Custom handlers | ❌ Fixed handler | ✅ Configurable |
| File output | ❌ Host-only | ✅ Direct |
| Performance | ⚠️ WASM overhead | ✅ Native |
| Centralized logs | ✅ Automatic | ⚠️ Manual setup |

## See Also

- [Go slog Documentation](https://pkg.go.dev/log/slog)
- [Main SDK Documentation](../README.md)
