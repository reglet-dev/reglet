// Package main provides a TCP plugin for Reglet.
// This is compiled to WASM and loaded by the Reglet runtime.
//
// Uses Go 1.24+ //go:wasmexport directive for function exports.
//go:build wasip1

package main

import (
	"encoding/json"
	"fmt"
	"unsafe"
)

// Import host function for TCP connections
//
//go:wasmimport reglet_host tcp_connect
func host_tcp_connect(hostPtr, hostLen, portPtr, portLen, timeoutMs, useTLS uint32) uint32

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

	buf := make([]byte, size)
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

// Helper: copyToMemory writes data to WASM linear memory at the given pointer.
func copyToMemory(ptr uint32, data []byte) {
	dest := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), len(data))
	copy(dest, data)
}

// Helper: readFromMemory reads data from WASM linear memory at the given pointer.
func readFromMemory(ptr uint32, length uint32) []byte {
	src := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), length)
	data := make([]byte, length)
	copy(data, src)
	return data
}

// Helper: marshalToPtr marshals data to JSON and returns a packed pointer+length.
func marshalToPtr(data interface{}) uint64 {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return 0
	}

	ptr := allocate(uint32(len(jsonData)))
	copyToMemory(ptr, jsonData)
	return packPtrLen(ptr, uint32(len(jsonData)))
}

// packPtrLen packs pointer and length into a single uint64.
func packPtrLen(ptr uint32, length uint32) uint64 {
	return (uint64(ptr) << 32) | uint64(length)
}

// Helper: successResponse creates a successful observation result.
func successResponse(data map[string]interface{}) uint64 {
	result := map[string]interface{}{"status": true}
	for key, value := range data {
		result[key] = value
	}
	return marshalToPtr(result)
}

// Helper: errorResponse creates an error observation result.
func errorResponse(message string) uint64 {
	result := map[string]interface{}{
		"status": false,
		"error":  message,
	}
	return marshalToPtr(result)
}

// describe returns plugin metadata as JSON in WASM memory.
//
//go:wasmexport describe
func describe() uint64 {
	info := map[string]interface{}{
		"name":        "tcp",
		"version":     "1.0.0",
		"description": "TCP connection testing and TLS validation",
		"capabilities": []map[string]string{
			{
				"kind":    "network",
				"pattern": "outbound:*",
			},
		},
	}

	return marshalToPtr(info)
}

// schema returns configuration schema as JSON.
//
//go:wasmexport schema
func schema() uint64 {
	configSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"host": map[string]interface{}{
				"type":        "string",
				"description": "Target host (hostname or IP)",
			},
			"port": map[string]interface{}{
				"type":        "string",
				"description": "Target port",
			},
			"timeout_ms": map[string]interface{}{
				"type":        "integer",
				"description": "Connection timeout in milliseconds",
				"default":     5000,
			},
			"tls": map[string]interface{}{
				"type":        "boolean",
				"description": "Use TLS/SSL connection",
				"default":     false,
			},
			"expected_tls_version": map[string]interface{}{
				"type":        "string",
				"description": "Expected minimum TLS version (e.g., 'TLS 1.2')",
			},
		},
		"required": []string{"host", "port"},
	}

	return marshalToPtr(configSchema)
}

// observe executes the observation with the given configuration.
//
//go:wasmexport observe
func observe(configPtr uint32, configLen uint32) uint64 {
	// Read config from WASM memory
	configData := readFromMemory(configPtr, configLen)

	var config map[string]interface{}
	if err := json.Unmarshal(configData, &config); err != nil {
		return errorResponse(fmt.Sprintf("failed to parse config: %v", err))
	}

	// Validate required fields
	host, ok := config["host"].(string)
	if !ok || host == "" {
		return errorResponse("missing required field: host")
	}

	port, ok := config["port"].(string)
	if !ok || port == "" {
		return errorResponse("missing required field: port")
	}

	// Get timeout (default to 5000ms)
	timeoutMs := uint32(5000)
	if t, ok := config["timeout_ms"].(float64); ok {
		timeoutMs = uint32(t)
	}

	// Get TLS flag (default to false)
	useTLS := uint32(0)
	if tls, ok := config["tls"].(bool); ok && tls {
		useTLS = 1
	} else if tlsStr, ok := config["tls"].(string); ok && (tlsStr == "true" || tlsStr == "1") {
		useTLS = 1
	}

	// Perform TCP connection test
	response, err := performTCPConnect(host, port, timeoutMs, useTLS)
	if err != nil {
		return errorResponse(fmt.Sprintf("TCP connection failed: %v", err))
	}

	// Check TLS version expectation if provided
	if expectedTLSVersion, ok := config["expected_tls_version"].(string); ok {
		if actualVersion, ok := response["tls_version"].(string); ok {
			if !isTLSVersionAtLeast(actualVersion, expectedTLSVersion) {
				response["expectation_failed"] = true
				response["expectation_error"] = fmt.Sprintf("expected TLS version >= %s, got %s",
					expectedTLSVersion, actualVersion)
				return successResponse(response)
			}
		}
	}

	// Return success response
	return successResponse(response)
}

// performTCPConnect executes the TCP connection test by calling the host function
func performTCPConnect(host, port string, timeoutMs, useTLS uint32) (map[string]interface{}, error) {
	// Allocate memory for host
	hostPtr := allocate(uint32(len(host)))
	copyToMemory(hostPtr, []byte(host))
	defer deallocate(hostPtr, uint32(len(host)))

	// Allocate memory for port
	portPtr := allocate(uint32(len(port)))
	copyToMemory(portPtr, []byte(port))
	defer deallocate(portPtr, uint32(len(port)))

	// Call host function
	resultPtr := host_tcp_connect(
		hostPtr, uint32(len(host)),
		portPtr, uint32(len(port)),
		timeoutMs,
		useTLS,
	)

	// Read result from WASM memory
	maxSize := uint32(64 * 1024)
	resultDataFull := readFromMemory(resultPtr, maxSize)

	// Trim null bytes
	var resultData []byte
	for i, b := range resultDataFull {
		if b == 0 {
			resultData = resultDataFull[:i]
			break
		}
	}
	if resultData == nil {
		resultData = resultDataFull
	}

	defer deallocate(resultPtr, maxSize)

	// Parse JSON result
	var result map[string]interface{}
	if err := json.Unmarshal(resultData, &result); err != nil {
		return nil, fmt.Errorf("failed to parse host function result: %v", err)
	}

	// Check if operation succeeded
	status, ok := result["status"].(bool)
	if !ok || !status {
		errMsg, _ := result["error"].(string)
		if errMsg == "" {
			errMsg = "unknown error from host function"
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	return result, nil
}

// isTLSVersionAtLeast checks if actual TLS version meets the minimum requirement
func isTLSVersionAtLeast(actual, minimum string) bool {
	versions := map[string]int{
		"TLS 1.0": 10,
		"TLS 1.1": 11,
		"TLS 1.2": 12,
		"TLS 1.3": 13,
	}

	actualVal, okActual := versions[actual]
	minimumVal, okMinimum := versions[minimum]

	if !okActual || !okMinimum {
		return false
	}

	return actualVal >= minimumVal
}

func main() {
	// Required for WASM, but never called
}
