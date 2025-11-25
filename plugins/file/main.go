// Package main provides a file plugin for Reglet that checks file existence and content.
// This is compiled to WASM and loaded by the Reglet runtime.
package main

import (
	"fmt"
	"os"
)

// This is a simple file plugin that checks file existence and reads content
// It will be compiled to WASM and loaded by the Reglet runtime

// TODO: Use wit-bindgen-go to generate proper bindings
// For now, this is a placeholder structure to understand what we need

// describe returns plugin metadata
//
//export describe
//
//nolint:unused // WASM export - called by host
func describe() {
	// TODO: Implement proper WIT response
	// Should return PluginInfo with:
	// - name: "file"
	// - version: "1.0.0"
	// - description: "File existence and content checks"
	// - capabilities: [{ kind: "fs", pattern: "read:**" }]
	fmt.Println("file plugin describe called")
}

// schema returns configuration schema
//
//export schema
//
//nolint:unused // WASM export - called by host
func schema() {
	// TODO: Implement proper WIT response
	// Should return ConfigSchema with fields:
	// - path (string, required): Path to file
	// - mode (string, optional): Check mode (exists, readable, content)
	fmt.Println("file plugin schema called")
}

// observe executes the file check
//
//export observe
//
//nolint:unused // WASM export - called by host
func observe() {
	// TODO: Implement proper WIT request/response
	// Should:
	// 1. Parse Config from WASM memory
	// 2. Extract path from config
	// 3. Check if file exists
	// 4. Read file content if needed
	// 5. Return Evidence with results
	fmt.Println("file plugin observe called")
}

// observeWithPath is a simpler version that takes a path parameter
// This is easier to test before we implement full WIT bindings
//
//export observeWithPath
//
//nolint:unused // WASM export - called by host
func observeWithPath(pathPtr, pathLen int32) int32 {
	// Read path from WASM memory
	// For now, just demonstrate the concept
	fmt.Printf("observing file at path ptr=%d len=%d\n", pathPtr, pathLen)

	// In a real implementation:
	// 1. Read string from WASM memory using pathPtr and pathLen
	// 2. Check if file exists: os.Stat(path)
	// 3. Read file if needed: os.ReadFile(path)
	// 4. Marshal result to WASM memory
	// 5. Return pointer to result

	return 0 // Success
}

// fileExists checks if a file exists
//
//nolint:unused // Will be used when WIT bindings are implemented
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// main is required for WASM compilation but won't be called
// The host calls exported functions directly
func main() {
	// This function is never called in WASM mode
	// Exported functions are called directly by the host
}
