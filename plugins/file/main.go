// Package main provides a file plugin for Reglet that checks file existence and content.
// This is compiled to WASM and loaded by the Reglet runtime.
//
// Uses Go 1.24+ //go:wasmexport directive for function exports.
package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"unsafe"
)

// This is a file plugin that checks file existence and reads content.
// It's compiled to WASM using: GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared

// Memory management for passing data between host and plugin.
// allocations keeps a reference to allocated memory to prevent the GC from collecting it.
// This effectively "pins" the memory until the host explicitly releases it.
var allocations = make(map[uint32][]byte)

// allocate reserves memory in the WASM linear memory and returns a pointer.
// The host can read from this pointer.
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

	// PIN THE MEMORY: Store the slice in a global map so the GC sees it as "in use"
	allocations[ptr] = buf

	return ptr
}

// deallocate frees memory by removing the reference, allowing the GC to collect it.
//
//go:wasmexport deallocate
func deallocate(ptr uint32, size uint32) {
	delete(allocations, ptr)
}

// describe returns plugin metadata as JSON in WASM memory.
// Returns a pointer to the JSON-encoded PluginInfo.
//
//go:wasmexport describe
func describe() uint64 {
	info := map[string]interface{}{
		"name":        "file",
		"version":     "1.0.0",
		"description": "File existence and content checks",
		"capabilities": []map[string]string{
			{
				"kind":    "fs",
				"pattern": "read:**",
			},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(info)
	if err != nil {
		return 0 // Error
	}

	// Allocate memory and copy data
	ptr := allocate(uint32(len(data)))
	copyToMemory(ptr, data)
	return packPtrLen(ptr, uint32(len(data)))
}

// schema returns configuration schema as JSON.
// Returns a pointer to the JSON-encoded ConfigSchema.
//
//go:wasmexport schema
func schema() uint64 {
	configSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to file to check",
			},
			"mode": map[string]interface{}{
				"type":        "string",
				"description": "Check mode: exists, readable, or content",
				"enum":        []string{"exists", "readable", "content"},
				"default":     "exists",
			},
		},
		"required": []string{"path"},
	}

	data, err := json.Marshal(configSchema)
	if err != nil {
		return 0
	}

	ptr := allocate(uint32(len(data)))
	copyToMemory(ptr, data)
	return packPtrLen(ptr, uint32(len(data)))
}

// observe executes the file check.
// Takes a pointer to JSON config, returns pointer to JSON result.
//
//go:wasmexport observe
func observe(configPtr uint32, configLen uint32) uint64 {
	// Read config from WASM memory
	config := readFromMemory(configPtr, configLen)

	// Parse config
	var cfg map[string]interface{}
	if err := json.Unmarshal(config, &cfg); err != nil {
		return errorResult("failed to parse config: " + err.Error())
	}

	path, ok := cfg["path"].(string)
	if !ok {
		return errorResult("path field is required and must be a string")
	}

	mode := "exists"
	if m, ok := cfg["mode"].(string); ok {
		mode = m
	}

	// Execute the check
	var result map[string]interface{}

	switch mode {
	case "exists":
		_, err := os.Stat(path)
		result = map[string]interface{}{
			"status": err == nil,
			"path":   path,
			"mode":   mode,
		}
		if err != nil {
			result["error"] = err.Error()
		}

	case "readable":
		file, err := os.Open(path)
		if err == nil {
			file.Close()
			result = map[string]interface{}{
				"status": true,
				"path":   path,
				"mode":   mode,
			}
		} else {
			result = map[string]interface{}{
				"status": false,
				"path":   path,
				"mode":   mode,
				"error":  err.Error(),
			}
		}

	case "content":
		content, err := os.ReadFile(path)
		if err == nil {
			result = map[string]interface{}{
				"status":      true,
				"path":        path,
				"mode":        mode,
				"content_b64": base64.StdEncoding.EncodeToString(content),
				"encoding":    "base64",
				"size":        len(content),
			}
		} else {
			result = map[string]interface{}{
				"status": false,
				"path":   path,
				"mode":   mode,
				"error":  err.Error(),
			}
		}

	default:
		return errorResult("invalid mode: " + mode)
	}

	// Marshal result to JSON
	data, err := json.Marshal(result)
	if err != nil {
		return errorResult("failed to marshal result: " + err.Error())
	}

	ptr := allocate(uint32(len(data)))
	copyToMemory(ptr, data)
	return packPtrLen(ptr, uint32(len(data)))
}

// Helper functions

// copyToMemory copies data to WASM linear memory at the given pointer.
func copyToMemory(ptr uint32, data []byte) {
	dest := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), len(data))
	copy(dest, data)
}

// readFromMemory reads data from WASM linear memory.
func readFromMemory(ptr uint32, length uint32) []byte {
	src := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), length)
	data := make([]byte, length)
	copy(data, src)
	return data
}

// packPtrLen packs pointer and length into a single uint64.
// Pointer in high 32 bits, length in low 32 bits.
func packPtrLen(ptr uint32, length uint32) uint64 {
	return (uint64(ptr) << 32) | uint64(length)
}

// errorResult creates an error result and returns a pointer to it.
func errorResult(message string) uint64 {
	result := map[string]interface{}{
		"status": false,
		"error":  message,
	}
	data, _ := json.Marshal(result)
	ptr := allocate(uint32(len(data)))
	copyToMemory(ptr, data)
	return packPtrLen(ptr, uint32(len(data)))
}

// main is required for WASM compilation but won't be called
// The host calls exported functions directly
func main() {
	// This function is never called in WASM mode
	// Exported functions are called directly by the host
}
