# ABI Package

The `abi` package implements the Application Binary Interface (ABI) for WASM plugins. It handles memory management, pointer packing/unpacking, and data transfer across the WASM boundary between the plugin (guest) and the host.

## Overview

This package is **internal** and should not be used directly by plugin authors. It provides low-level primitives used by other SDK packages (`net`, `exec`, `log`) to communicate with the host.

## Key Concepts

### WASM Linear Memory

WASM modules have a single contiguous block of memory called **linear memory**. All data exchange between the plugin and host happens through this memory:

- **Guest (Plugin)**: Allocates memory and writes data
- **Host**: Reads from guest-allocated memory or writes to guest-allocated memory

### Pointer Packing

Since WASM functions can only pass/return integers, we pack pointers and lengths into a single `uint64`:

```
High 32 bits: Memory pointer (uint32)
Low 32 bits:  Data length (uint32)
```

This allows a single return value to convey both the location and size of data.

## Memory Management

### Allocation Tracking

The ABI tracks all memory allocations to prevent Go's garbage collector from prematurely collecting memory that the host still needs:

```go
var memoryManager = struct {
    sync.Mutex
    ptrs           map[uint32][]byte // ptr -> slice reference
    totalAllocated int               // Total bytes currently allocated
}{
    ptrs:           make(map[uint32][]byte),
    totalAllocated: 0,
}
```

### Memory Limit

To prevent unbounded memory growth, the ABI enforces a **100 MB limit**:

```go
const MaxTotalAllocations = 100 * 1024 * 1024 // 100 MB
```

If allocations exceed this limit, the plugin panics with a clear error message.

## Exported WASM Functions

These functions are exported to the host and implement the WASM memory protocol:

### allocate

```go
//go:wasmexport allocate
func allocate(size uint32) uint32
```

Allocates memory in WASM linear memory and returns a pointer:

- **Called by**: Host (when passing data to plugin)
- **Returns**: Pointer to allocated memory
- **Tracking**: Allocation is tracked to prevent GC

### deallocate

```go
//go:wasmexport deallocate
func deallocate(ptr uint32, size uint32)
```

Frees previously allocated memory:

- **Called by**: Host (after reading data from plugin)
- **Effect**: Removes tracking, allows GC to collect
- **Safety**: Updates `totalAllocated` counter

## SDK Functions

These functions are used by other SDK packages but not directly by plugin authors:

### PtrFromBytes

```go
func PtrFromBytes(data []byte) uint64
```

Allocates WASM memory, copies data into it, and returns packed pointer/length:

```go
// Used internally by SDK packages
jsonData := []byte(`{"key": "value"}`)
packed := abi.PtrFromBytes(jsonData)
// Returns: packed uint64 containing pointer and length
```

**Usage Flow:**
1. Allocate memory with `allocate()`
2. Copy `data` into allocated memory
3. Pack pointer and length into `uint64`
4. Return packed value to caller

### BytesFromPtr

```go
func BytesFromPtr(packed uint64) []byte
```

Unpacks a uint64 and reads the corresponding data from WASM memory:

```go
// Host calls plugin function, passes packed pointer
packed := hostFunction()
data := abi.BytesFromPtr(packed)
// Returns: []byte containing the data
```

**Usage Flow:**
1. Unpack `uint64` into pointer and length
2. Read data from WASM linear memory at that pointer
3. Return copy of data as `[]byte`

### DeallocatePacked

```go
func DeallocatePacked(packed uint64)
```

Unpacks a uint64 and deallocates the memory:

```go
// After host reads data from plugin
packed := plugin_function()
data := abi.BytesFromPtr(packed)
abi.DeallocatePacked(packed) // Free memory allocated by plugin
```

**Usage Flow:**
1. Unpack `uint64` into pointer and length
2. Call `deallocate()` to free memory
3. Decrement `totalAllocated` counter

### PackPtrLen

```go
func PackPtrLen(ptr, length uint32) uint64
```

Packs a pointer and length into a single `uint64`:

```go
ptr := uint32(0x1000)
length := uint32(256)
packed := abi.PackPtrLen(ptr, length)
// Returns: 0x0000100000000100
```

**Panics**: If `ptr == 0` and `length > 0` (invalid state)

### UnpackPtrLen

```go
func UnpackPtrLen(packed uint64) (ptr, length uint32)
```

Unpacks a `uint64` into pointer and length:

```go
packed := uint64(0x0000100000000100)
ptr, length := abi.UnpackPtrLen(packed)
// Returns: ptr=0x1000, length=256
```

**Panics**: If `ptr == 0` and `length > 0` (invalid packed value)

### FreeAllTracked

```go
func FreeAllTracked()
```

Frees all currently tracked allocations:

```go
// Called during panic recovery
defer func() {
    if r := recover() {
        abi.FreeAllTracked() // Clean up all allocations
        panic(r)
    }
}()
```

**Usage:**
- Called automatically during panic recovery in `plugin.go`
- Resets `totalAllocated` to 0
- Prevents memory leaks on error paths

## Memory Protocol

### Guest → Host (Plugin sends data to host)

```go
// 1. Plugin prepares data
data := []byte(`{"result": "success"}`)

// 2. Allocate and copy to WASM memory
packed := abi.PtrFromBytes(data)

// 3. Return packed pointer to host
return packed

// 4. Host reads data from pointer
// 5. Host calls deallocate(ptr, len)
```

### Host → Guest (Host sends data to plugin)

```go
// 1. Host calls guest's allocate(size)
// 2. Guest returns pointer
// 3. Host writes data to that pointer
// 4. Host calls guest function with packed pointer/length
func guestFunction(packed uint64) {
    // 5. Guest reads data
    data := abi.BytesFromPtr(packed)

    // 6. Guest deallocates after processing
    defer abi.DeallocatePacked(packed)

    // Process data...
}
```

## Memory Safety

### Pointer Validation

The ABI validates pointer/length combinations to prevent invalid memory access:

```go
func PackPtrLen(ptr, length uint32) uint64 {
    if ptr == 0 && length > 0 {
        panic("abi: invalid pack - null pointer with non-zero length")
    }
    return (uint64(ptr) << 32) | uint64(length)
}
```

This prevents:
- ❌ Null pointer with non-zero length
- ❌ Reading from invalid memory locations

### Allocation Limits

The ABI enforces strict memory limits:

```go
if memoryManager.totalAllocated+int(size) > MaxTotalAllocations {
    panic(fmt.Sprintf("abi: memory allocation limit exceeded"))
}
```

This prevents:
- ❌ Memory exhaustion attacks
- ❌ Unbounded memory growth
- ❌ Out-of-memory errors

### Double-Free Protection

The deallocation logic prevents negative counters from double-free:

```go
memoryManager.totalAllocated -= int(size)
if memoryManager.totalAllocated < 0 {
    memoryManager.totalAllocated = 0 // Prevent negative
}
```

## Unsafe Operations

The ABI uses `unsafe.Pointer` for direct memory access:

```go
//nolint:gosec // G103: Valid unsafe.Pointer use for WASM linear memory access
dest := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), len(data))
copy(dest, data)
```

**Why Safe:**
- WASM linear memory is a single contiguous array
- Pointer arithmetic is valid within this array
- `uint32` pointers map directly to WASM memory offsets
- `unsafe` is necessary for performance (no reflection overhead)

**Linting:**
- `//nolint:gosec` annotations suppress security warnings
- These are reviewed and confirmed safe for WASM context

## Example: Complete Data Flow

Here's how data flows through the ABI when a plugin makes an HTTP request:

```go
// 1. Plugin prepares HTTP request
request := HTTPRequestWire{
    Method: "GET",
    URL:    "https://example.com",
}

// 2. Marshal to JSON
requestBytes, _ := json.Marshal(request)

// 3. Allocate WASM memory and copy data
packedRequest := abi.PtrFromBytes(requestBytes)
// - Calls allocate(len(requestBytes))
// - Tracks allocation in memoryManager
// - Returns packed pointer/length

// 4. Call host function
packedResponse := host_http_request(packedRequest)

// 5. Read response from host-allocated memory
responseBytes := abi.BytesFromPtr(packedResponse)
// - Unpacks pointer and length
// - Reads from WASM memory
// - Returns copy of data

// 6. Free host-allocated memory
abi.DeallocatePacked(packedResponse)
// - Removes from memoryManager
// - Decrements totalAllocated

// 7. Unmarshal response
var response HTTPResponseWire
json.Unmarshal(responseBytes, &response)
```

## Performance Considerations

1. **Zero-Copy Reads**: `BytesFromPtr` creates a copy to ensure safety after deallocation
2. **Memory Pooling**: Not implemented (Go GC handles this efficiently)
3. **Lock Contention**: `sync.Mutex` protects allocations (WASM is single-threaded, minimal contention)
4. **Allocation Overhead**: Tracking map has ~40 bytes overhead per allocation

## Debugging

### Memory Leak Detection

Check current allocation count:

```go
import "log/slog"

slog.Info("Memory stats",
    "total_allocated", memoryManager.totalAllocated,
    "num_allocations", len(memoryManager.ptrs),
)
```

### Allocation Tracking

Enable debug logging to track all allocations:

```go
func allocate(size uint32) uint32 {
    slog.Debug("Allocating memory", "size", size, "total", memoryManager.totalAllocated)
    // ... rest of function
}
```

## Limitations

1. **100 MB Limit**: Total allocations capped at 100 MB
2. **Single-Threaded**: WASM is single-threaded, mutex is defensive
3. **No Memory Pooling**: Each allocation goes through Go's allocator
4. **Copy Overhead**: `BytesFromPtr` always copies data for safety

## See Also

- [Main SDK Documentation](../../README.md)
- [Plugin Development Guide](../../../../docs/plugin-development.md)
