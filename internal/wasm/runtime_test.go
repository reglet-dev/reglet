package wasm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRuntime(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	runtime, err := NewRuntime(ctx)
	require.NoError(t, err)
	require.NotNil(t, runtime)

	// Verify runtime is initialized
	assert.NotNil(t, runtime.runtime)
	assert.NotNil(t, runtime.plugins)
	assert.Empty(t, runtime.plugins)

	// Clean up
	err = runtime.Close(ctx)
	assert.NoError(t, err)
}

func TestRuntime_GetPlugin_NotLoaded(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	runtime, err := NewRuntime(ctx)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	// Try to get a plugin that hasn't been loaded
	plugin, ok := runtime.GetPlugin("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, plugin)
}

// TestLoadPlugin_InvalidWASM tests loading invalid WASM bytes
func TestLoadPlugin_InvalidWASM(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	runtime, err := NewRuntime(ctx)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	// Try to load invalid WASM
	invalidWasm := []byte("not a valid wasm module")
	plugin, err := runtime.LoadPlugin(ctx, "invalid", invalidWasm)

	assert.Error(t, err)
	assert.Nil(t, plugin)
	assert.Contains(t, err.Error(), "failed to compile")
}

// TODO: Add test with actual valid WASM module
// This requires building a simple plugin first
