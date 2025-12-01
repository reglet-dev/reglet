// Package main provides a dns plugin for Reglet.
// This is compiled to WASM and loaded by the Reglet runtime.
//
// Uses Go 1.24+ //go:wasmexport directive for function exports.
//go:build wasip1

package main

import (
	"encoding/json"
	"fmt"
	"time"
	"unsafe"
)

// Import host function for DNS lookups
//
//go:wasmimport reglet_host dns_lookup
func host_dns_lookup(hostnamePtr, hostnameLen, recordTypePtr, recordTypeLen uint32) uint32

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
		"name":        "dns",
		"version":     "1.0.0",
		"description": "DNS resolution and record validation",
		"capabilities": []map[string]string{
			{
				"kind":    "network",
				"pattern": "outbound:53",
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
			"hostname": map[string]interface{}{
				"type":        "string",
				"description": "Hostname to resolve",
			},
			"record_type": map[string]interface{}{
				"type":        "string",
				"description": "DNS record type to query",
				"enum":        []string{"A", "AAAA", "CNAME", "MX", "TXT", "NS"},
				"default":     "A",
			},
			"nameserver": map[string]interface{}{
				"type":        "string",
				"description": "Custom nameserver (optional, e.g., 8.8.8.8:53)",
			},
		},
		"required": []string{"hostname"},
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
	hostname, ok := config["hostname"].(string)
	if !ok || hostname == "" {
		return errorResponse("missing required field: hostname")
	}

	// Get record type (default to A)
	recordType := "A"
	if rt, ok := config["record_type"].(string); ok && rt != "" {
		recordType = rt
	}

	// Get custom nameserver if provided
	var nameserver string
	if ns, ok := config["nameserver"].(string); ok {
		nameserver = ns
	}

	// Perform DNS lookup
	start := time.Now()
	records, err := performLookup(hostname, recordType, nameserver)
	queryTime := time.Since(start).Milliseconds()

	if err != nil {
		return errorResponse(fmt.Sprintf("DNS lookup failed: %v", err))
	}

	// Return success response with records
	return successResponse(map[string]interface{}{
		"hostname":      hostname,
		"record_type":   recordType,
		"records":       records,
		"record_count":  len(records),
		"query_time_ms": queryTime,
	})
}

// performLookup executes the DNS lookup by calling the host function
// TODO: Support custom nameserver parameter in host function (Phase 2D)
func performLookup(hostname, recordType, nameserver string) ([]string, error) {
	// Ignore nameserver for now - host function uses default resolver
	_ = nameserver

	// Allocate memory for hostname
	hostnamePtr := allocate(uint32(len(hostname)))
	copyToMemory(hostnamePtr, []byte(hostname))
	defer deallocate(hostnamePtr, uint32(len(hostname)))

	// Allocate memory for record type
	recordTypePtr := allocate(uint32(len(recordType)))
	copyToMemory(recordTypePtr, []byte(recordType))
	defer deallocate(recordTypePtr, uint32(len(recordType)))

	// Call host function
	resultPtr := host_dns_lookup(hostnamePtr, uint32(len(hostname)), recordTypePtr, uint32(len(recordType)))

	// Read result from WASM memory
	// First, find the actual length by reading until we find the end of JSON or null byte
	// For now, read a reasonable max and then trim to actual JSON length
	maxSize := uint32(64 * 1024)
	resultDataFull := readFromMemory(resultPtr, maxSize)

	// Find the actual end of the JSON by looking for the closing brace and trimming null bytes
	// This is a workaround - ideally the host function should return the length too
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

	defer deallocate(resultPtr, maxSize) // Deallocate what was allocated

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

	// Extract records from result
	recordsInterface, ok := result["records"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid records format in host function result")
	}

	// Convert []interface{} to []string
	records := make([]string, len(recordsInterface))
	for i, r := range recordsInterface {
		records[i], _ = r.(string)
	}

	return records, nil
}

func main() {
	// Required for WASM, but never called
}
