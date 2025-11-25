package wasm

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadFilePlugin tests loading the actual file plugin WASM module
func TestLoadFilePlugin(t *testing.T) {
	// Skip if WASM file doesn't exist
	wasmPath := filepath.Join("..", "..", "plugins", "file", "file.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("file.wasm not built - run 'make -C plugins/file build' first")
	}

	// Read the WASM file
	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)
	require.NotEmpty(t, wasmBytes)

	// Create runtime
	ctx := context.Background()
	runtime, err := NewRuntime(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	// Load the plugin
	plugin, err := runtime.LoadPlugin("file", wasmBytes)
	require.NoError(t, err)
	require.NotNil(t, plugin)

	// Verify plugin is cached
	cachedPlugin, ok := runtime.GetPlugin("file")
	assert.True(t, ok)
	assert.Equal(t, plugin, cachedPlugin)

	// Verify plugin name
	assert.Equal(t, "file", plugin.Name())
}

// TestFilePlugin_Describe tests calling the describe function
// Uses Go 1.24+ //go:wasmexport for function exports
func TestFilePlugin_Describe(t *testing.T) {
	// Skip if WASM file doesn't exist
	wasmPath := filepath.Join("..", "..", "plugins", "file", "file.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("file.wasm not built - run 'make -C plugins/file build' first")
	}

	// Read the WASM file
	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	// Create runtime and load plugin
	ctx := context.Background()
	runtime, err := NewRuntime(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	plugin, err := runtime.LoadPlugin("file", wasmBytes)
	require.NoError(t, err)

	// Call describe and verify real plugin metadata
	info, err := plugin.Describe()
	require.NoError(t, err)
	require.NotNil(t, info)

	// Verify plugin metadata matches what the plugin exports
	assert.Equal(t, "file", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Equal(t, "File existence and content checks", info.Description)

	// Verify capabilities
	require.Len(t, info.Capabilities, 1)
	assert.Equal(t, "fs", info.Capabilities[0].Kind)
	assert.Equal(t, "read:**", info.Capabilities[0].Pattern)
}

// TestFilePlugin_Schema tests calling the schema function
func TestFilePlugin_Schema(t *testing.T) {
	// Skip if WASM file doesn't exist
	wasmPath := filepath.Join("..", "..", "plugins", "file", "file.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("file.wasm not built - run 'make -C plugins/file build' first")
	}

	// Read the WASM file
	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	// Create runtime and load plugin
	ctx := context.Background()
	runtime, err := NewRuntime(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	plugin, err := runtime.LoadPlugin("file", wasmBytes)
	require.NoError(t, err)

	// Call schema and verify we get valid JSON Schema
	schema, err := plugin.Schema()
	require.NoError(t, err)
	require.NotNil(t, schema)
	require.NotEmpty(t, schema.RawSchema)

	// Parse the JSON Schema to verify it's valid JSON
	var jsonSchema map[string]interface{}
	err = json.Unmarshal(schema.RawSchema, &jsonSchema)
	require.NoError(t, err)

	// Verify it's valid JSON Schema with expected fields
	require.NotEmpty(t, jsonSchema)
	assert.Equal(t, "object", jsonSchema["type"])

	// Verify properties exist
	props, ok := jsonSchema["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, props, "path")
	assert.Contains(t, props, "mode")
}

// TestFilePlugin_Observe_FileExists tests checking if a file exists
func TestFilePlugin_Observe_FileExists(t *testing.T) {
	// Skip if WASM file doesn't exist
	wasmPath := filepath.Join("..", "..", "plugins", "file", "file.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("file.wasm not built - run 'make -C plugins/file build' first")
	}

	// Create a temporary test file
	tmpFile, err := os.CreateTemp("", "reglet-test-*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString("test content")
	tmpFile.Close()

	// Read the WASM file
	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	// Create runtime and load plugin
	ctx := context.Background()
	runtime, err := NewRuntime(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	plugin, err := runtime.LoadPlugin("file", wasmBytes)
	require.NoError(t, err)

	// Test file exists check
	config := Config{
		Values: map[string]string{
			"path": tmpFile.Name(),
			"mode": "exists",
		},
	}

	result, err := plugin.Observe(config)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.Error)
	require.NotNil(t, result.Evidence)

	// Verify the file was found
	status, ok := result.Evidence.Data["status"].(bool)
	require.True(t, ok)
	assert.True(t, status)
	assert.Equal(t, tmpFile.Name(), result.Evidence.Data["path"])
	assert.Equal(t, "exists", result.Evidence.Data["mode"])
}

// TestFilePlugin_Observe_FileNotFound tests checking a non-existent file
func TestFilePlugin_Observe_FileNotFound(t *testing.T) {
	// Skip if WASM file doesn't exist
	wasmPath := filepath.Join("..", "..", "plugins", "file", "file.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("file.wasm not built - run 'make -C plugins/file build' first")
	}

	// Read the WASM file
	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	// Create runtime and load plugin
	ctx := context.Background()
	runtime, err := NewRuntime(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	plugin, err := runtime.LoadPlugin("file", wasmBytes)
	require.NoError(t, err)

	// Test non-existent file
	config := Config{
		Values: map[string]string{
			"path": "/tmp/reglet-nonexistent-file-12345.txt",
			"mode": "exists",
		},
	}

	result, err := plugin.Observe(config)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.Error, "file not found should return evidence with status=false, not an error")
	require.NotNil(t, result.Evidence)

	// Verify the file was not found
	status, ok := result.Evidence.Data["status"].(bool)
	require.True(t, ok)
	assert.False(t, status, "status should be false for non-existent file")
	assert.Contains(t, result.Evidence.Data, "error", "should include error message in evidence")

	// Verify the error message makes sense
	errMsg, ok := result.Evidence.Data["error"].(string)
	require.True(t, ok)
	assert.Contains(t, errMsg, "such file", "error message should indicate file not found")
}

// TestFilePlugin_Observe_ReadContent tests reading file content
func TestFilePlugin_Observe_ReadContent(t *testing.T) {
	// Skip if WASM file doesn't exist
	wasmPath := filepath.Join("..", "..", "plugins", "file", "file.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skip("file.wasm not built - run 'make -C plugins/file build' first")
	}

	// Create a temporary test file with known content
	tmpFile, err := os.CreateTemp("", "reglet-test-*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	testContent := "Hello from Reglet!"
	tmpFile.WriteString(testContent)
	tmpFile.Close()

	// Read the WASM file
	wasmBytes, err := os.ReadFile(wasmPath)
	require.NoError(t, err)

	// Create runtime and load plugin
	ctx := context.Background()
	runtime, err := NewRuntime(ctx)
	require.NoError(t, err)
	defer runtime.Close()

	plugin, err := runtime.LoadPlugin("file", wasmBytes)
	require.NoError(t, err)

	// Test content reading
	config := Config{
		Values: map[string]string{
			"path": tmpFile.Name(),
			"mode": "content",
		},
	}

	result, err := plugin.Observe(config)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.Error)
	require.NotNil(t, result.Evidence)

	// Verify the content was read
	status, ok := result.Evidence.Data["status"].(bool)
	require.True(t, ok)
	assert.True(t, status)

	content, ok := result.Evidence.Data["content"].(string)
	require.True(t, ok)
	assert.Equal(t, testContent, content)

	// Verify size
	size, ok := result.Evidence.Data["size"].(float64) // JSON numbers are float64
	require.True(t, ok)
	assert.Equal(t, float64(len(testContent)), size)
}
