//go:build wasip1

package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime/debug" // For stack traces in panic recovery

	"github.com/whiskeyjimbo/reglet/sdk/internal/abi"
)

// Plugin is the interface every Reglet plugin must implement.
type Plugin interface {
	// Describe returns metadata about the plugin.
	Describe(ctx context.Context) (Metadata, error)
	// Schema returns the JSON schema for the plugin's configuration.
	Schema(ctx context.Context) ([]byte, error)
	// Check executes the plugin's main logic with the given configuration.
	Check(ctx context.Context, config Config) (Evidence, error)
}

// Internal variable to hold the user's plugin implementation.
var userPlugin Plugin

// Register initializes the WASM exports and handles the plugin lifecycle.
// Plugin authors call this in their `main()` function.
func Register(p Plugin) {
	if userPlugin != nil {
		slog.Warn("sdk: plugin already registered, ignoring second call", "userPlugin_addr", fmt.Sprintf("%p", &userPlugin))
		return
	}
	userPlugin = p
	slog.Info("sdk: plugin registered successfully", "userPlugin_addr", fmt.Sprintf("%p", &userPlugin))
}

// Define the functions that will be exported to the WASM host.
// These functions perform panic recovery and ABI translation.

//go:wasmexport describe
func _describe() uint64 {
	return handleExportedCall(func() (interface{}, error) {
		if userPlugin == nil {
			return nil, fmt.Errorf("plugin not registered")
		}
		// Context propagation is for a later phase, using Background for now.
		metadata, err := userPlugin.Describe(context.Background())
		if err != nil {
			return nil, err
		}
		// Auto-populate SDK version for metadata
		metadata.SDKVersion = Version
		metadata.MinHostVersion = MinHostVersion
		return metadata, nil
	})
}

//go:wasmexport schema
func _schema() uint64 {
	return handleExportedCall(func() (interface{}, error) {
		slog.Debug("sdk: _schema called", "userPlugin_addr", fmt.Sprintf("%p", &userPlugin), "userPlugin_nil", userPlugin == nil)
		if userPlugin == nil {
			return nil, fmt.Errorf("plugin not registered")
		}
		// Context propagation is for a later phase, using Background for now.
		schemaBytes, err := userPlugin.Schema(context.Background())
		if err != nil {
			return nil, err
		}
		// Schema is raw JSON bytes, so return as is
		return schemaBytes, nil
	})
}

//go:wasmexport observe
func _observe(configPtr uint32, configLen uint32) uint64 {
	return handleExportedCall(func() (interface{}, error) {
		slog.Debug("sdk: _observe called", "userPlugin_addr", fmt.Sprintf("%p", &userPlugin), "userPlugin_nil", userPlugin == nil)
		if userPlugin == nil {
			return nil, fmt.Errorf("plugin not registered")
		}

		// Read config from WASM memory
		configBytes := abi.BytesFromPtr(abi.PackPtrLen(configPtr, configLen))
		var config Config
		if err := json.Unmarshal(configBytes, &config); err != nil {
			return nil, fmt.Errorf("failed to parse config: %w", err)
		}

		// Context propagation is for a later phase, using Background for now.
		evidence, err := userPlugin.Check(context.Background(), config)
		if err != nil {
			// If user's check returns an error, embed it in Evidence
			evidence.Status = false
			evidence.Error = ToErrorDetail(err)
		}
		return evidence, nil
	})
}

// handleExportedCall is a generic wrapper for WASM exported functions.
// It provides panic recovery, error handling, and JSON serialization.
// It ensures that on any error or panic, a structured Evidence with ErrorDetail is returned.
func handleExportedCall(f func() (interface{}, error)) (packedResult uint64) {
	// Use a named return parameter to ensure it's set before `panic` is propagated.
	defer func() {
		if r := recover(); r != nil {
			// Free all tracked allocations on panic to prevent leaks.
			abi.FreeAllTracked()

			errDetail := &ErrorDetail{
				Message: fmt.Sprintf("plugin panic: %v", r),
				Type:    "panic",
				Stack:   debug.Stack(), // Capture stack trace for panics
			}
			slog.Error("sdk: plugin panic recovered", "error", errDetail.Message)
			packedResult = packEvidenceWithError(Evidence{Status: false, Error: errDetail})
		}
	}()

	result, err := f()
	if err != nil {
		slog.Error("sdk: plugin function returned error", "error", err.Error())
		packedResult = packEvidenceWithError(Evidence{Status: false, Error: ToErrorDetail(err)})
		return
	}

	var dataToMarshal []byte
	switch v := result.(type) {
	case []byte: // For Schema() returning raw JSON bytes
		dataToMarshal = v
	default:
		var marshalErr error
		dataToMarshal, marshalErr = json.Marshal(v)
		if marshalErr != nil {
			slog.Error("sdk: failed to marshal result", "error", marshalErr.Error())
			packedResult = packEvidenceWithError(Evidence{Status: false, Error: ToErrorDetail(marshalErr)})
			return
		}
	}

	packedResult = abi.PtrFromBytes(dataToMarshal)
	return
}

// packEvidenceWithError marshals an Evidence struct (containing an error) to JSON
// and returns the packed pointer/length. Used for internal SDK errors/panics.
func packEvidenceWithError(ev Evidence) uint64 {
	data, err := json.Marshal(ev)
	if err != nil {
		// Fallback if even marshaling the error fails
		slog.Error("sdk: critical - failed to marshal error evidence", "original_error", ev.Error.Message, "marshal_error", err.Error())
		fallbackErr := &ErrorDetail{Message: "sdk: critical error during error marshalling", Type: "internal"}
		data, _ = json.Marshal(Evidence{Status: false, Error: fallbackErr}) // Try to marshal a generic error
	}
	return abi.PtrFromBytes(data)
}
