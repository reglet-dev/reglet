package wasm

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/reglet-dev/reglet/internal/infrastructure/build"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRuntime_ConcurrentPluginAccess verifies that the Runtime can safely handle
// concurrent calls to LoadPlugin, GetPlugin, and GetPluginSchema.
// This test must pass with -race detector.
func TestRuntime_ConcurrentPluginAccess(t *testing.T) {
	ctx := context.Background()
	version := build.Info{Version: "test", Commit: "abc123", BuildDate: "2024-01-01"}

	runtime, err := NewRuntime(ctx, version)
	require.NoError(t, err)
	defer func() {
		_ = runtime.Close(ctx)
	}()

	// Load a real WASM plugin file for testing
	pluginPath := filepath.Join("..", "..", "..", "plugins", "file", "file.wasm")
	wasmBytes, err := os.ReadFile(pluginPath)
	require.NoError(t, err, "Failed to read file plugin. Run 'cd plugins/file && make build' first")

	const (
		numGoroutines = 50
		pluginName    = "file"
	)

	// Test concurrent LoadPlugin calls (should only compile once)
	t.Run("concurrent LoadPlugin", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				plugin, err := runtime.LoadPlugin(ctx, pluginName, wasmBytes)
				assert.NoError(t, err)
				assert.NotNil(t, plugin)
				assert.Equal(t, pluginName, plugin.name)
			}()
		}

		wg.Wait()

		// Verify only one plugin instance exists
		plugin, ok := runtime.GetPlugin(pluginName)
		assert.True(t, ok)
		assert.NotNil(t, plugin)
	})

	// Test concurrent GetPlugin calls
	t.Run("concurrent GetPlugin", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				plugin, ok := runtime.GetPlugin(pluginName)
				assert.True(t, ok)
				assert.NotNil(t, plugin)
				assert.Equal(t, pluginName, plugin.name)
			}()
		}

		wg.Wait()
	})

	// Test concurrent GetPluginSchema calls
	t.Run("concurrent GetPluginSchema", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				schema, err := runtime.GetPluginSchema(ctx, pluginName)
				assert.NoError(t, err)
				// Schema may be nil if plugin doesn't provide one
				_ = schema
			}()
		}

		wg.Wait()
	})

	// Test mixed concurrent operations
	t.Run("mixed concurrent operations", func(t *testing.T) {
		const mixedPluginName = "file-concurrent"
		var wg sync.WaitGroup
		wg.Add(numGoroutines * 3) // 3 types of operations

		// LoadPlugin
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				plugin, err := runtime.LoadPlugin(ctx, mixedPluginName, wasmBytes)
				assert.NoError(t, err)
				assert.NotNil(t, plugin)
			}()
		}

		// GetPlugin
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				_, _ = runtime.GetPlugin(mixedPluginName)
				// May return false if LoadPlugin hasn't completed yet
			}()
		}

		// GetPluginSchema
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				_, _ = runtime.GetPluginSchema(ctx, mixedPluginName)
				// May return error if LoadPlugin hasn't completed yet
			}()
		}

		wg.Wait()

		// Verify plugin was eventually loaded
		plugin, ok := runtime.GetPlugin(mixedPluginName)
		assert.True(t, ok)
		assert.NotNil(t, plugin)
	})
}

// TestRuntime_ConcurrentLoadDifferentPlugins verifies that loading different
// plugins concurrently works correctly.
func TestRuntime_ConcurrentLoadDifferentPlugins(t *testing.T) {
	ctx := context.Background()
	version := build.Info{Version: "test", Commit: "abc123", BuildDate: "2024-01-01"}

	runtime, err := NewRuntime(ctx, version)
	require.NoError(t, err)
	defer func() {
		_ = runtime.Close(ctx)
	}()

	// Load a real WASM plugin file for testing
	pluginPath := filepath.Join("..", "..", "..", "plugins", "file", "file.wasm")
	wasmBytes, err := os.ReadFile(pluginPath)
	require.NoError(t, err, "Failed to read file plugin. Run 'cd plugins/file && make build' first")

	const numPlugins = 10

	var wg sync.WaitGroup
	wg.Add(numPlugins)

	for i := 0; i < numPlugins; i++ {
		pluginName := filepath.Join("plugin", string(rune('a'+i)))
		go func(name string) {
			defer wg.Done()
			plugin, err := runtime.LoadPlugin(ctx, name, wasmBytes)
			assert.NoError(t, err)
			assert.NotNil(t, plugin)
			assert.Equal(t, name, plugin.name)
		}(pluginName)
	}

	wg.Wait()

	// Verify all plugins were loaded
	for i := 0; i < numPlugins; i++ {
		pluginName := filepath.Join("plugin", string(rune('a'+i)))
		plugin, ok := runtime.GetPlugin(pluginName)
		assert.True(t, ok, "Plugin %s should be loaded", pluginName)
		assert.NotNil(t, plugin)
	}
}
