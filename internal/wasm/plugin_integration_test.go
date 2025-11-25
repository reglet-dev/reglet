package wasm

import (
	"context"
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
// NOTE: Currently skipped because standard Go WASM doesn't export functions properly
// This will work once we switch to TinyGo for plugin compilation
func TestFilePlugin_Describe(t *testing.T) {
	t.Skip("Standard Go WASM doesn't export functions - need TinyGo")

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

	// Call describe
	// Note: This will return placeholder data until we implement WIT bindings
	info, err := plugin.Describe()
	require.NoError(t, err)
	require.NotNil(t, info)

	// For now, just verify the placeholder values
	assert.Equal(t, "file", info.Name)
	// TODO: Once WIT bindings are implemented, verify real plugin metadata
}
