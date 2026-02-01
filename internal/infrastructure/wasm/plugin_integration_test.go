package wasm

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/reglet-dev/reglet/internal/infrastructure/build"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Global cache for WASM bytes
var (
	fixtureWasmBytes []byte
	initOnce         sync.Once
)

func getFixtureWasm(t *testing.T) []byte {
	initOnce.Do(func() {
		// Assume running from internal/infrastructure/wasm
		path := filepath.Join("testdata", "fixture.wasm")
		bytes, err := os.ReadFile(path)
		if err == nil {
			fixtureWasmBytes = bytes
		}
	})

	if len(fixtureWasmBytes) == 0 {
		// Try to read again in case init failed implicitly
		path := filepath.Join("testdata", "fixture.wasm")
		bytes, err := os.ReadFile(path)
		require.NoError(t, err, "failed to read fixture.wasm")
		fixtureWasmBytes = bytes
	}

	return fixtureWasmBytes
}

func TestHost_LoadFixture(t *testing.T) {
	t.Parallel()
	wasmBytes := getFixtureWasm(t)

	ctx := context.Background()
	runtime, err := NewRuntime(ctx, build.Get())
	require.NoError(t, err)
	defer runtime.Close(ctx)

	plugin, err := runtime.LoadPlugin(ctx, "fixture", wasmBytes)
	require.NoError(t, err)
	require.NotNil(t, plugin)

	assert.Equal(t, "fixture", plugin.Name())
}

func TestHost_Manifest(t *testing.T) {
	t.Parallel()
	wasmBytes := getFixtureWasm(t)

	ctx := context.Background()
	runtime, err := NewRuntime(ctx, build.Get())
	require.NoError(t, err)
	defer runtime.Close(ctx)

	plugin, err := runtime.LoadPlugin(ctx, "fixture", wasmBytes)
	require.NoError(t, err)

	manifest, err := plugin.Manifest(ctx)
	require.NoError(t, err)
	require.NotNil(t, manifest)

	assert.Equal(t, "fixture", manifest.Name)
	assert.Equal(t, "0.0.1", manifest.Version)
	assert.NotEmpty(t, manifest.Capabilities)

	// Verify specific capability from fixture
	found := false
	for _, cap := range manifest.Capabilities {
		if cap.Category == "env" && cap.Resource == "TEST_VAR" {
			found = true
			break
		}
	}
	assert.True(t, found, "should find TEST_VAR env capability")
}

func TestHost_Observe_Success(t *testing.T) {
	t.Parallel()
	wasmBytes := getFixtureWasm(t)

	ctx := context.Background()
	runtime, err := NewRuntime(ctx, build.Get()) // No caps needed for basic echo
	require.NoError(t, err)
	defer runtime.Close(ctx)

	plugin, err := runtime.LoadPlugin(ctx, "fixture", wasmBytes)
	require.NoError(t, err)

	config := Config{
		Values: map[string]interface{}{
			"action": "echo",
			"input":  "hello world",
		},
	}

	result, err := plugin.Observe(ctx, config)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.Error)
	require.NotNil(t, result.Evidence)

	assert.True(t, result.Evidence.Status, "status should be true")
	assert.Equal(t, "hello world", result.Evidence.Data["echo"])
}

func TestHost_Observe_Failure(t *testing.T) {
	t.Parallel()
	wasmBytes := getFixtureWasm(t)

	ctx := context.Background()
	runtime, err := NewRuntime(ctx, build.Get())
	require.NoError(t, err)
	defer runtime.Close(ctx)

	plugin, err := runtime.LoadPlugin(ctx, "fixture", wasmBytes)
	require.NoError(t, err)

	config := Config{
		Values: map[string]interface{}{
			"action": "fail",
		},
	}

	result, err := plugin.Observe(ctx, config)
	require.NoError(t, err) // SDK returns success but Evidence has failure status
	// Wait, Observe wrapper might return error if Evidence.Error is set?
	// The current plugin.Observe implementation in host (plugin.go) returns *PluginObservationResult.
	// It doesn't auto-convert Evidence failure to Go error unless internally logic does so.
	// Let's check result content.

	assert.False(t, result.Evidence.Status, "status should be false")
	require.NotNil(t, result.Evidence.Error, "evidence error should be set")
	assert.Equal(t, "requested failure", result.Evidence.Error.Message)
}

func TestHost_Schema(t *testing.T) {
	t.Parallel()
	wasmBytes := getFixtureWasm(t)

	ctx := context.Background()
	runtime, err := NewRuntime(ctx, build.Get())
	require.NoError(t, err)
	defer runtime.Close(ctx)

	plugin, err := runtime.LoadPlugin(ctx, "fixture", wasmBytes)
	require.NoError(t, err)

	manifest, err := plugin.Manifest(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, manifest.ConfigSchema)

	var schema map[string]interface{}
	err = json.Unmarshal(manifest.ConfigSchema, &schema)
	require.NoError(t, err)

	props, ok := schema["properties"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, props, "action")
}
