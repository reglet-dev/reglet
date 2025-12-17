package wasm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/build"
)

func TestNewRuntime(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	runtime, err := NewRuntime(ctx, build.Get())
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
	runtime, err := NewRuntime(ctx, build.Get())
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
	runtime, err := NewRuntime(ctx, build.Get())
	require.NoError(t, err)
	defer runtime.Close(ctx)

	// Try to load invalid WASM
	invalidWasm := []byte("not a valid wasm module")
	plugin, err := runtime.LoadPlugin(ctx, "invalid", invalidWasm)

	assert.Error(t, err)
	assert.Nil(t, plugin)
	assert.Contains(t, err.Error(), "failed to compile")
}

func TestNewRuntime_DefaultMemoryLimit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	// 0 means default
	runtime, err := NewRuntimeWithCapabilities(ctx, build.Get(), nil, nil, 0)
	require.NoError(t, err)
	defer runtime.Close(ctx)
	assert.NotNil(t, runtime)
}

func TestNewRuntime_ExplicitMemoryLimit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	// Explicit limit (100MB)
	runtime, err := NewRuntimeWithCapabilities(ctx, build.Get(), nil, nil, 100)
	require.NoError(t, err)
	defer runtime.Close(ctx)
	assert.NotNil(t, runtime)
}

func TestNewRuntime_UnlimitedMemoryLimit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	// -1 means unlimited
	runtime, err := NewRuntimeWithCapabilities(ctx, build.Get(), nil, nil, -1)
	require.NoError(t, err)
	defer runtime.Close(ctx)
	assert.NotNil(t, runtime)
}

func TestNewRuntime_InvalidMemoryLimit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	// < -1 is invalid
	runtime, err := NewRuntimeWithCapabilities(ctx, build.Get(), nil, nil, -2)
	assert.Error(t, err)
	assert.Nil(t, runtime)
	assert.Contains(t, err.Error(), "invalid WASM memory limit")
}

// TODO: Add test with actual valid WASM module
// This requires building a simple plugin first
