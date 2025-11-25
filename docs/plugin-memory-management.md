# Plugin Memory Management Guide

## Overview

When writing WASM plugins for Reglet in Go, proper memory management is **critical** to prevent memory corruption and ensure reliable operation. This guide documents the correct pattern for managing memory between the host (Reglet) and guest (WASM plugin).

## The Problem: Go GC and WASM Memory

When a Go WASM plugin allocates memory and returns a pointer to the host, the Go Garbage Collector (GC) doesn't know the host still needs that memory. Without keeping a reference, the GC will reclaim it, causing the host to read garbage data.

### Symptoms of Improper Memory Management

- `invalid character '\u00a0' after object key:value pair` when parsing JSON
- Random memory corruption after 2-3 plugin calls
- Works once, then fails on subsequent calls
- Non-deterministic failures based on GC timing

## The Solution: Memory Pinning

**Pin allocated memory** by storing references in a global map until the host explicitly calls `deallocate()`.

## Correct Implementation Pattern

### In Your Plugin (`plugins/*/main.go`)

```go
package main

import (
    "encoding/json"
    "os"
    "unsafe"
)

// CRITICAL: Global map to pin allocated memory and prevent GC collection
var allocations = make(map[uint32][]byte)

// allocate reserves memory and returns a pointer to the host.
// The memory is "pinned" by storing it in the allocations map.
//
//go:wasmexport allocate
func allocate(size uint32) uint32 {
    if size == 0 {
        return 0
    }

    // Allocate the slice
    buf := make([]byte, size)

    // Get the pointer to the underlying array
    ptr := uint32(uintptr(unsafe.Pointer(&buf[0])))

    // PIN THE MEMORY: Store the slice so GC sees it as "in use"
    allocations[ptr] = buf

    return ptr
}

// deallocate frees memory by removing the reference.
// This allows the GC to collect it.
//
//go:wasmexport deallocate
func deallocate(ptr uint32, size uint32) {
    delete(allocations, ptr)
}

// Helper: Copy data to allocated memory
func copyToMemory(ptr uint32, data []byte) {
    dest := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), len(data))
    copy(dest, data)
}

// Helper: Read data from memory
func readFromMemory(ptr uint32, length uint32) []byte {
    src := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), length)
    data := make([]byte, length)
    copy(data, src)
    return data
}

// Example: describe() function that returns JSON
//
//go:wasmexport describe
func describe() uint32 {
    info := map[string]interface{}{
        "name":    "example",
        "version": "1.0.0",
    }

    // Marshal to JSON
    data, err := json.Marshal(info)
    if err != nil {
        return 0
    }

    // Allocate memory and copy data
    ptr := allocate(uint32(len(data)))
    copyToMemory(ptr, data)

    // Host will read from ptr and then call deallocate(ptr, len)
    return ptr
}

// Example: observe() function that takes config and returns result
//
//go:wasmexport observe
func observe(configPtr uint32, configLen uint32) uint32 {
    // Read config from WASM memory
    config := readFromMemory(configPtr, configLen)

    var cfg map[string]interface{}
    if err := json.Unmarshal(config, &cfg); err != nil {
        return errorResult("failed to parse config: " + err.Error())
    }

    // Do your observation work...
    result := map[string]interface{}{
        "status": true,
        "data":   "example",
    }

    // Marshal and return result
    data, _ := json.Marshal(result)
    ptr := allocate(uint32(len(data)))
    copyToMemory(ptr, data)
    return ptr
}

func errorResult(message string) uint32 {
    result := map[string]interface{}{
        "status": false,
        "error":  message,
    }
    data, _ := json.Marshal(result)
    ptr := allocate(uint32(len(data)))
    copyToMemory(ptr, data)
    return ptr
}

func main() {
    // Required for WASM, but never called
}
```

### In the Host (`internal/wasm/plugin.go`)

The host must:
1. Call `allocate()` to get memory for writing data to the plugin
2. Call `deallocate()` after the plugin has read that data
3. Call `deallocate()` after reading data returned by the plugin

```go
// writeToMemory allocates memory in the WASM module and writes data to it
func (p *Plugin) writeToMemory(instance api.Module, data []byte) (uint32, error) {
    allocateFn := instance.ExportedFunction("allocate")
    if allocateFn == nil {
        return 0, fmt.Errorf("plugin does not export allocate() function")
    }

    // Allocate memory for the data
    results, err := allocateFn.Call(p.ctx, uint64(len(data)))
    if err != nil {
        return 0, fmt.Errorf("failed to allocate memory: %w", err)
    }

    ptr := uint32(results[0])
    if ptr == 0 {
        return 0, fmt.Errorf("allocate() returned null pointer")
    }

    // Write data to the allocated memory
    if !instance.Memory().Write(ptr, data) {
        return 0, fmt.Errorf("failed to write to WASM memory at offset %d", ptr)
    }

    return ptr, nil
}

// readString reads data from WASM memory and deallocates it
func (p *Plugin) readString(instance api.Module, ptr uint32) ([]byte, error) {
    // CRITICAL: Use defer to ensure memory is ALWAYS deallocated, even on error
    // This prevents memory leaks in long-running processes
    defer func() {
        deallocateFn := instance.ExportedFunction("deallocate")
        if deallocateFn != nil {
            // Use maxSize as safe upper bound - plugin's deallocate handles actual size
            deallocateFn.Call(p.ctx, uint64(ptr), uint64(64*1024))
        }
    }()

    maxSize := uint32(64 * 1024)

    data, ok := instance.Memory().Read(ptr, maxSize)
    if !ok {
        return nil, fmt.Errorf("failed to read memory at offset %d", ptr)
    }

    // Find null terminator
    end := 0
    for i, b := range data {
        if b == 0 {
            end = i
            break
        }
    }

    var result []byte
    if end == 0 {
        result = make([]byte, len(data))
        copy(result, data)
    } else {
        result = make([]byte, end)
        copy(result, data[:end])
    }

    return result, nil
}

// Example: Calling observe() with proper memory management
func (p *Plugin) Observe(cfg Config) (*ObservationResult, error) {
    instance, err := p.getInstance()
    if err != nil {
        return nil, err
    }

    observeFn := instance.ExportedFunction("observe")
    if observeFn == nil {
        return nil, fmt.Errorf("plugin does not export observe() function")
    }

    // Marshal config to JSON
    configData, err := json.Marshal(cfg.Values)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal config: %w", err)
    }

    // Write config to WASM memory
    configPtr, err := p.writeToMemory(instance, configData)
    if err != nil {
        return nil, fmt.Errorf("failed to write config: %w", err)
    }

    // CRITICAL: Use defer to ensure config memory is ALWAYS deallocated
    // Even if observe() fails or panics, we must clean up
    defer func() {
        deallocateFn := instance.ExportedFunction("deallocate")
        if deallocateFn != nil {
            deallocateFn.Call(p.ctx, uint64(configPtr), uint64(len(configData)))
        }
    }()

    // Call observe(configPtr, configLen)
    results, err := observeFn.Call(p.ctx, uint64(configPtr), uint64(len(configData)))
    if err != nil {
        return nil, fmt.Errorf("failed to call observe(): %w", err)
    }

    resultPtr := uint32(results[0])
    if resultPtr == 0 {
        return nil, fmt.Errorf("observe() returned null pointer")
    }

    // Read result (readString will deallocate resultPtr)
    resultData, err := p.readString(instance, resultPtr)
    if err != nil {
        return nil, fmt.Errorf("failed to read result: %w", err)
    }

    // Parse and return result
    var rawResult map[string]interface{}
    if err := json.Unmarshal(resultData, &rawResult); err != nil {
        return nil, fmt.Errorf("failed to parse result: %w", err)
    }

    return &ObservationResult{
        Evidence: &Evidence{
            Data: rawResult,
        },
    }, nil
}
```

## WASI Filesystem Configuration

For plugins that need filesystem access (like the file plugin), the host must properly configure WASI:

```go
func (p *Plugin) getInstance() (api.Module, error) {
    if p.instance != nil {
        return p.instance, nil
    }

    // Configure WASI with filesystem access
    config := wazero.NewModuleConfig().
        // Mount host root "/" to guest root "/"
        WithFSConfig(wazero.NewFSConfig().WithDirMount("/", "/")).
        // Enable time-related syscalls (needed for file timestamps)
        WithSysWalltime().
        WithSysNanotime().
        WithSysNanosleep().
        // Enable random number generation
        WithRandSource(rand.Reader)

    instance, err := p.runtime.InstantiateModule(p.ctx, p.module, config)
    if err != nil {
        return nil, fmt.Errorf("failed to instantiate: %w", err)
    }

    // Call _initialize for WASI modules built with -buildmode=c-shared
    initFn := instance.ExportedFunction("_initialize")
    if initFn != nil {
        if _, err := initFn.Call(p.ctx); err != nil {
            instance.Close(p.ctx)
            return nil, fmt.Errorf("failed to initialize: %w", err)
        }
    }

    p.instance = instance
    return instance, nil
}
```

## Key Principles

1. **Always pin allocations**: Store slices in a global map to prevent GC collection
2. **Always use defer for deallocation**: Ensures cleanup even on errors or panics
3. **Never return early without cleanup**: Use defer immediately after allocation
4. **Copy defensive**: Make copies when reading/writing across the WASM boundary
5. **Handle errors**: Return `0` pointer on allocation failures
6. **WASI requires mounting**: Use `WithFSConfig().WithDirMount()` for filesystem access

### Why defer is Critical

Without `defer`, any error return path will leak memory:

```go
// ❌ BAD: Leaks memory if observe() fails
configPtr, _ := p.writeToMemory(instance, configData)
results, err := observeFn.Call(p.ctx, configPtr, len(configData))
if err != nil {
    return nil, err  // LEAKED: configPtr never deallocated!
}
deallocateFn.Call(p.ctx, configPtr, len(configData))

// ✅ GOOD: Always deallocates, even on error
configPtr, _ := p.writeToMemory(instance, configData)
defer func() {
    deallocateFn.Call(p.ctx, configPtr, len(configData))
}()
results, err := observeFn.Call(p.ctx, configPtr, len(configData))
if err != nil {
    return nil, err  // OK: defer will clean up
}
```

For a CLI tool that runs for seconds, leaks don't matter. For a long-running daemon or server, leaks are **fatal**.

## Memory Lifecycle

```
1. Host calls plugin.allocate(size) → Plugin returns ptr
2. Plugin stores buf in allocations[ptr] (pinned)
3. Host writes data to ptr via Memory.Write()
4. Host calls plugin function (e.g., observe(ptr, len))
5. Plugin reads from ptr, processes, allocates result
6. Plugin returns resultPtr
7. Host reads from resultPtr via Memory.Read()
8. Host calls plugin.deallocate(ptr, len) for input
9. Host calls plugin.deallocate(resultPtr, len) for output
10. Plugin removes entries from allocations map
11. GC can now collect the memory
```

## Testing Memory Management

Always test with:
- Multiple sequential plugin calls (3+)
- Different data sizes
- Race detector: `go test -race ./...`
- Long-running tests to expose GC timing issues

## Common Mistakes

❌ **DON'T**: Return pointer without pinning
```go
func allocate(size uint32) uint32 {
    buf := make([]byte, size)
    return uint32(uintptr(unsafe.Pointer(&buf[0]))) // GC will reclaim this!
}
```

✅ **DO**: Pin with global map
```go
var allocations = make(map[uint32][]byte)

func allocate(size uint32) uint32 {
    buf := make([]byte, size)
    ptr := uint32(uintptr(unsafe.Pointer(&buf[0])))
    allocations[ptr] = buf // Pinned!
    return ptr
}
```

❌ **DON'T**: Forget to deallocate
```go
// Host reads result but never calls deallocate
resultData, _ := p.readString(instance, resultPtr)
// MEMORY LEAK!
```

✅ **DO**: Always deallocate after reading
```go
resultData, _ := p.readString(instance, resultPtr)
// readString internally calls deallocate(resultPtr)
```

## References

- [File Plugin Implementation](../plugins/file/main.go) - Reference implementation
- [Host Plugin Wrapper](../internal/wasm/plugin.go) - Host-side memory management
- [wazero Documentation](https://wazero.io/) - WASM runtime details
- [WASI Filesystem](https://wazero.io/languages/go/#wasi) - Filesystem configuration

## History

This pattern was discovered and documented during Phase 1b when investigating memory corruption issues:
- **Issue**: `invalid character '\u00a0'` errors on third observation
- **Root Cause**: Go GC reclaiming memory before host could read it
- **Solution**: Global allocations map to pin memory + proper deallocate calls
- **Result**: All observations now work reliably, including directory stat operations

**Date**: November 25, 2025
**Commit**: See git log for `fix(wasm): implement proper memory pinning`
