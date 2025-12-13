package wasm

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPlugin_Observe_Concurrent tests that multiple goroutines can call
// Observe() concurrently without data races or cross-contamination
func TestPlugin_Observe_Concurrent(t *testing.T) {
	ctx := context.Background()
	runtime, err := NewRuntime(ctx)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	// Load file plugin
	wasmBytes, err := os.ReadFile("../../../plugins/file/file.wasm")
	require.NoError(t, err)

	plugin, err := runtime.LoadPlugin(ctx, "file", wasmBytes)
	require.NoError(t, err)

	// Create multiple temp files with unique content
	numFiles := 10
	files := make([]*os.File, numFiles)
	expectedPaths := make([]string, numFiles)

	for i := 0; i < numFiles; i++ {
		f, err := os.CreateTemp(".", "concurrent-test-*.txt")
		require.NoError(t, err)
		defer os.Remove(f.Name())

		content := fmt.Sprintf("Content %d", i)
		_, err = f.WriteString(content)
		require.NoError(t, err)
		f.Close()

		files[i] = f
		expectedPaths[i] = f.Name()
	}

	// Run concurrent observations
	var wg sync.WaitGroup
	results := make([]*ObservationResult, numFiles)

	for i := 0; i < numFiles; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			config := Config{
				Values: map[string]interface{}{
					"path": expectedPaths[idx],
					"mode": "exists",
				},
			}

			result, err := plugin.Observe(ctx, config)
			require.NoError(t, err)
			results[idx] = result
		}(i)
	}

	wg.Wait()

	// Verify all results are correct (no cross-contamination)
	for i, result := range results {
		assert.NotNil(t, result, "Result %d should not be nil", i)
		assert.NotNil(t, result.Evidence, "Evidence for result %d should not be nil", i)

		// Verify Evidence status
		assert.True(t, result.Evidence.Status, "Result %d Evidence.Status should be true", i)
		require.Nil(t, result.Evidence.Error, "Result %d Evidence.Error should be nil", i)

		path, ok := result.Evidence.Data["path"].(string)
		require.True(t, ok, "Result %d should have path field", i)
		assert.Equal(t, expectedPaths[i], path, "Result %d path should match", i)
	}
}

// TestPlugin_Observe_RaceDetector is explicitly for running with -race flag
// to detect any data races in the plugin system
func TestPlugin_Observe_RaceDetector(t *testing.T) {
	// This test runs the concurrent test specifically to catch race conditions
	// Run with: go test -race ./internal/wasm/... -run TestPlugin_Observe_RaceDetector
	TestPlugin_Observe_Concurrent(t)
}

// TestPlugin_ConcurrentDifferentMethods tests that different methods
// (Describe, Schema, Observe) can be called concurrently
func TestPlugin_ConcurrentDifferentMethods(t *testing.T) {
	ctx := context.Background()
	runtime, err := NewRuntime(ctx)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	// Load file plugin
	wasmBytes, err := os.ReadFile("../../../plugins/file/file.wasm")
	require.NoError(t, err)

	plugin, err := runtime.LoadPlugin(ctx, "file", wasmBytes)
	require.NoError(t, err)

	// Create a test file
	tmpFile, err := os.CreateTemp(".", "method-test-*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString("Test content")
	tmpFile.Close()

	var wg sync.WaitGroup
	errors := make([]error, 3)

	// Call Describe() in goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := plugin.Describe(ctx)
		errors[0] = err
	}()

	// Call Schema() in goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := plugin.Schema(ctx)
		errors[1] = err
	}()

	// Call Observe() in goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		config := Config{
			Values: map[string]interface{}{
				"path": tmpFile.Name(),
				"mode": "exists",
			},
		}
		_, err := plugin.Observe(ctx, config)
		errors[2] = err
	}()

	wg.Wait()

	// Verify all methods succeeded
	for i, err := range errors {
		assert.NoError(t, err, "Method %d should not error", i)
	}
}
