//go:build wasip1

package abi

import (
	"fmt"
	"sync"
	"unsafe"
)

// MemoryManager tracks all allocations made by the SDK in WASM linear memory.
// It keeps a reference to allocated slices to prevent the Go GC from collecting them,
// effectively "pinning" the memory until explicitly freed or during panic recovery.
var memoryManager = struct {
	sync.Mutex
	ptrs map[uint32][]byte // ptr -> slice reference
}{ptrs: make(map[uint32][]byte)}

// allocate reserves memory in the WASM linear memory and returns a pointer.
// The host can read from this pointer. It tracks the allocation to prevent GC.
//
//go:wasmexport allocate
func allocate(size uint32) uint32 {
	if size == 0 {
		return 0
	}

	buf := make([]byte, size)
	ptr := uint32(uintptr(unsafe.Pointer(&buf[0])))

	memoryManager.Lock()
	memoryManager.ptrs[ptr] = buf // PIN THE MEMORY: Store the slice to prevent GC
	memoryManager.Unlock()

	return ptr
}

// deallocate frees memory by removing the reference from the memory manager,
// allowing the Go GC to collect it.
//
//go:wasmexport deallocate
func deallocate(ptr uint32, size uint32) {
	memoryManager.Lock()
	delete(memoryManager.ptrs, ptr)
	memoryManager.Unlock()
}

// FreeAllTracked frees all memory currently tracked by the SDK.
// This is typically called during panic recovery or module shutdown to prevent leaks.
func FreeAllTracked() {
	memoryManager.Lock()
	for ptr := range memoryManager.ptrs {
		delete(memoryManager.ptrs, ptr)
	}
	memoryManager.Unlock()
}

// PtrFromBytes allocates WASM memory, copies the given data into it,
// and returns the packed pointer and length (uint64).
// The allocated memory is tracked by the SDK for later deallocation.
// This is used when the Guest (plugin) sends data to the Host.
func PtrFromBytes(data []byte) uint64 {
	if len(data) == 0 {
		return 0
	}
	size := uint32(len(data))
	ptr := allocate(size)
	copyToMemory(ptr, data)
	return PackPtrLen(ptr, size)
}

// BytesFromPtr unpacks a uint64 into a pointer and length, then reads
// the corresponding data from WASM linear memory.
// The memory must have been allocated by the Host for the Guest to read.
// This is used when the Guest receives data from the Host.
func BytesFromPtr(packed uint64) []byte {
	ptr, length := UnpackPtrLen(packed)
	if ptr == 0 || length == 0 {
		return nil
	}
	return readFromMemory(ptr, length)
}

// DeallocatePacked unpacks a uint64 pointer/length and deallocates the memory.
// This is used to free memory allocated by the Guest but passed to the Host
// after the Host is done with it (if the protocol requires Guest to own it),
// OR more commonly, to free memory allocated by the HOST that was passed to Guest?
// Actually, if Host calls Guest export `allocate`, Guest owns it.
// If Guest calls Host function, Guest allocates args. Host reads them.
// Guest should free args after call returns.
func DeallocatePacked(packed uint64) {
	ptr, length := UnpackPtrLen(packed)
	if ptr != 0 && length > 0 {
		deallocate(ptr, length)
	}
}

// PackPtrLen packs a pointer and length into a single uint64.
// Pointer is stored in the high 32 bits, length in the low 32 bits.
// Panics if ptr is 0 and length > 0, indicating an invalid state.
func PackPtrLen(ptr, length uint32) uint64 {
	if ptr == 0 && length > 0 {
		panic(fmt.Sprintf("abi: invalid pack - null pointer (0x0) with non-zero length (%d)", length))
	}
	return (uint64(ptr) << 32) | uint64(length)
}

// UnpackPtrLen unpacks a uint64 into its original pointer and length.
// Panics if ptr is 0 and length > 0, indicating an invalid packed value.
func UnpackPtrLen(packed uint64) (ptr, length uint32) {
	ptr = uint32(packed >> 32)
	length = uint32(packed)
	if ptr == 0 && length > 0 {
		panic(fmt.Sprintf("abi: invalid unpack - null pointer (0x0) with non-zero length (%d)", length))
	}
	return ptr, length
}

// copyToMemory copies data to WASM linear memory at the given pointer.
func copyToMemory(ptr uint32, data []byte) {
	// The length of the slice (len(data)) must not exceed `size` when allocate was called.
	dest := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), len(data))
	copy(dest, data)
}

// readFromMemory reads data from WASM linear memory.
func readFromMemory(ptr uint32, length uint32) []byte {
	src := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), length)
	data := make([]byte, length) // Create a new slice to return a copy
	copy(data, src)
	return data
}