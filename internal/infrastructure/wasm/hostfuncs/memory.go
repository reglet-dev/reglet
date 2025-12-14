package hostfuncs

import (
	"context"
	"encoding/json"

	"github.com/tetratelabs/wazero/api"
)

// writeJSON marshals data to JSON and writes it to WASM memory
// Returns pointer to the allocated memory in WASM
//
//nolint:gosec // G115: uint64->uint32 conversions are safe for WASM32 address space
//nolint:unused // Helper functions for potential future use
func writeJSON(ctx context.Context, mod api.Module, data interface{}) uint32 {
	jsonData, err := json.Marshal(data)
	if err != nil {
		// Fallback to error response if marshaling fails
		return writeError(ctx, mod, "failed to marshal response")
	}

	// Call plugin's allocate function to get memory in WASM
	allocateFn := mod.ExportedFunction("allocate")
	if allocateFn == nil {
		// Can't allocate - return 0 (null pointer)
		return 0
	}

	results, err := allocateFn.Call(ctx, uint64(len(jsonData)))
	if err != nil || len(results) == 0 {
		return 0
	}

	ptr := uint32(results[0])

	// Write JSON data to WASM memory
	if !mod.Memory().Write(ptr, jsonData) {
		return 0
	}

	return ptr
}

// writeError creates an error response and writes it to WASM memory
func writeError(ctx context.Context, mod api.Module, message string) uint32 {
	return writeJSON(ctx, mod, map[string]interface{}{
		"status": false,
		"error":  message,
	})
}

// writeSuccess creates a success response with records and writes it to WASM memory
func writeSuccess(ctx context.Context, mod api.Module, records []string) uint32 {
	return writeJSON(ctx, mod, map[string]interface{}{
		"status":  true,
		"records": records,
	})
}
