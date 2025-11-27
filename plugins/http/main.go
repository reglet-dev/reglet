// Package main provides an HTTP plugin for Reglet.
// This is compiled to WASM and loaded by the Reglet runtime.
//
// Uses Go 1.24+ //go:wasmexport directive for function exports.
package main

import (
	"encoding/json"
	"fmt"
	"unsafe"
)

// Import host function for HTTP requests
//
//go:wasmimport reglet_host http_request
func host_http_request(urlPtr, urlLen, methodPtr, methodLen, headersPtr, headersLen, bodyPtr, bodyLen uint32) uint32

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

// Helper: marshalToPtr marshals data to JSON and returns a pointer to it.
func marshalToPtr(data interface{}) uint32 {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return 0
	}

	ptr := allocate(uint32(len(jsonData)))
	copyToMemory(ptr, jsonData)
	return ptr
}

// Helper: successResponse creates a successful observation result.
func successResponse(data map[string]interface{}) uint32 {
	result := map[string]interface{}{"status": true}
	for key, value := range data {
		result[key] = value
	}
	return marshalToPtr(result)
}

// Helper: errorResponse creates an error observation result.
func errorResponse(message string) uint32 {
	result := map[string]interface{}{
		"status": false,
		"error":  message,
	}
	return marshalToPtr(result)
}

// describe returns plugin metadata as JSON in WASM memory.
//
//go:wasmexport describe
func describe() uint32 {
	info := map[string]interface{}{
		"name":        "http",
		"version":     "1.0.0",
		"description": "HTTP/HTTPS request checking and validation",
		"capabilities": []map[string]string{
			{
				"kind":    "network",
				"pattern": "outbound:80,443",
			},
		},
	}

	return marshalToPtr(info)
}

// schema returns configuration schema as JSON.
//
//go:wasmexport schema
func schema() uint32 {
	configSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{
				"type":        "string",
				"description": "URL to request",
			},
			"method": map[string]interface{}{
				"type":        "string",
				"description": "HTTP method",
				"enum":        []string{"GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS", "PATCH"},
				"default":     "GET",
			},
			"body": map[string]interface{}{
				"type":        "string",
				"description": "Request body (for POST/PUT/PATCH)",
			},
			"expected_status": map[string]interface{}{
				"type":        "integer",
				"description": "Expected HTTP status code (optional)",
			},
			"expected_body_contains": map[string]interface{}{
				"type":        "string",
				"description": "String that should be present in response body (optional)",
			},
		},
		"required": []string{"url"},
	}

	return marshalToPtr(configSchema)
}

// observe executes the observation with the given configuration.
//
//go:wasmexport observe
func observe(configPtr uint32, configLen uint32) uint32 {
	// Read config from WASM memory
	configData := readFromMemory(configPtr, configLen)

	var config map[string]interface{}
	if err := json.Unmarshal(configData, &config); err != nil {
		return errorResponse(fmt.Sprintf("failed to parse config: %v", err))
	}

	// Validate required fields
	url, ok := config["url"].(string)
	if !ok || url == "" {
		return errorResponse("missing required field: url")
	}

	// Get method (default to GET)
	method := "GET"
	if m, ok := config["method"].(string); ok && m != "" {
		method = m
	}

	// Get request body (optional)
	body := ""
	if b, ok := config["body"].(string); ok {
		body = b
	}

	// Perform HTTP request
	response, err := performHTTPRequest(url, method, body)
	if err != nil {
		return errorResponse(fmt.Sprintf("HTTP request failed: %v", err))
	}

	// Check expectations if provided
	if expectedStatus, ok := config["expected_status"].(float64); ok {
		if int(expectedStatus) != response["status_code"].(int) {
			response["expectation_failed"] = true
			response["expectation_error"] = fmt.Sprintf("expected status %d, got %d",
				int(expectedStatus), response["status_code"].(int))
			return successResponse(response)
		}
	}

	if expectedContains, ok := config["expected_body_contains"].(string); ok {
		responseBody, _ := response["body"].(string)
		if !contains(responseBody, expectedContains) {
			response["expectation_failed"] = true
			response["expectation_error"] = fmt.Sprintf("expected body to contain '%s'", expectedContains)
			return successResponse(response)
		}
	}

	// Return success response
	return successResponse(response)
}

// performHTTPRequest executes the HTTP request by calling the host function
func performHTTPRequest(url, method, body string) (map[string]interface{}, error) {
	// Allocate memory for URL
	urlPtr := allocate(uint32(len(url)))
	copyToMemory(urlPtr, []byte(url))
	defer deallocate(urlPtr, uint32(len(url)))

	// Allocate memory for method
	methodPtr := allocate(uint32(len(method)))
	copyToMemory(methodPtr, []byte(method))
	defer deallocate(methodPtr, uint32(len(method)))

	// Allocate memory for body (if provided)
	var bodyPtr uint32
	var bodyLen uint32
	if body != "" {
		bodyLen = uint32(len(body))
		bodyPtr = allocate(bodyLen)
		copyToMemory(bodyPtr, []byte(body))
		defer deallocate(bodyPtr, bodyLen)
	}

	// Call host function (no custom headers for now)
	resultPtr := host_http_request(
		urlPtr, uint32(len(url)),
		methodPtr, uint32(len(method)),
		0, 0, // headers (not implemented yet)
		bodyPtr, bodyLen,
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

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || indexOfSubstring(s, substr) >= 0)
}

// indexOfSubstring returns the index of the first occurrence of substr in s, or -1
func indexOfSubstring(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	if len(substr) > len(s) {
		return -1
	}

	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func main() {
	// Required for WASM, but never called
}
