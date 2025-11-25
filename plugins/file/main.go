// Package main provides a file plugin for Reglet that checks file existence and content.
// This is compiled to WASM and loaded by the Reglet runtime.
//
// Uses Go 1.24+ //go:wasmexport directive for function exports.
package main

import (
	"encoding/json"
	"os"
	"unsafe"
)

// This is a file plugin that checks file existence and reads content.
// It's compiled to WASM using: GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared

// Memory management for passing data between host and plugin.
// We'll use a simple approach: allocate memory, return pointer to host.

// allocate reserves memory in the WASM linear memory and returns a pointer.
// The host can read from this pointer.
//
//go:wasmexport allocate
func allocate(size uint32) uint32 {
	// Allocate a byte slice of the requested size
	buf := make([]byte, size)
	// Return pointer to the first element
	return uint32(uintptr(unsafe.Pointer(&buf[0])))
}

// deallocate frees memory (no-op in Go due to GC, but kept for interface compatibility)
//
//go:wasmexport deallocate
func deallocate(ptr uint32, size uint32) {
	// Go's GC handles this, but we keep the function for API compatibility
}

// describe returns plugin metadata as JSON in WASM memory.
// Returns a pointer to the JSON-encoded PluginInfo.
//
//go:wasmexport describe
func describe() uint32 {
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
	return ptr
}

// schema returns configuration schema as JSON.
// Returns a pointer to the JSON-encoded ConfigSchema.
//
//go:wasmexport schema
func schema() uint32 {
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
	return ptr
}

// observe executes the file check.
// Takes a pointer to JSON config, returns pointer to JSON result.
//
//go:wasmexport observe
func observe(configPtr uint32, configLen uint32) uint32 {
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
				"status":  true,
				"path":    path,
				"mode":    mode,
				"content": string(content),
				"size":    len(content),
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
	return ptr
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

// errorResult creates an error result and returns a pointer to it.
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

// main is required for WASM compilation but won't be called
// The host calls exported functions directly
func main() {
	// This function is never called in WASM mode
	// Exported functions are called directly by the host
}
