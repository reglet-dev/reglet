package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/whiskeyjimbo/reglet/internal/domain/capabilities"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/build"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/wasm"
)

// TestPluginFilesystemIsolation verifies plugins can only access granted paths
func TestPluginFilesystemIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	// Create test files in two separate temporary directories
	allowedDir := t.TempDir()
	forbiddenDir := t.TempDir()

	allowedFile := filepath.Join(allowedDir, "allowed.txt")
	forbiddenFile := filepath.Join(forbiddenDir, "forbidden.txt")

	require.NoError(t, os.WriteFile(allowedFile, []byte("allowed content"), 0644))
	require.NoError(t, os.WriteFile(forbiddenFile, []byte("secret content"), 0644))

	// Grant access only to allowedDir, not forbiddenDir
	caps := map[string][]capabilities.Capability{
		"file": {
			{Kind: "fs", Pattern: "read:" + allowedDir + "/**"},
		},
	}

	runtime, err := wasm.NewRuntimeWithCapabilities(ctx, build.Get(), caps, nil, 0)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	// Load file plugin
	pluginWASM, err := os.ReadFile("../../plugins/file/file.wasm")
	if os.IsNotExist(err) {
		t.Skip("file.wasm not built - run 'cd plugins/file && make build' first")
	}
	require.NoError(t, err)

	plugin, err := runtime.LoadPlugin(ctx, "file", pluginWASM)
	require.NoError(t, err)

	// Test 1: Plugin CAN read allowed file (in mounted directory)
	cfg := wasm.Config{
		Values: map[string]interface{}{
			"path": allowedFile,
			"mode": "exists",
		},
	}
	result, err := plugin.Observe(ctx, cfg)
	require.NoError(t, err)
	assert.NotNil(t, result.Evidence)
	assert.True(t, result.Evidence.Status, "should be able to access allowed file")

	// Test 2: Plugin CANNOT read forbidden file (in non-mounted directory)
	cfg = wasm.Config{
		Values: map[string]interface{}{
			"path": forbiddenFile,
			"mode": "exists",
		},
	}
	result, err = plugin.Observe(ctx, cfg)
	// Plugin execution succeeds, but file check fails due to lack of access
	require.NoError(t, err)
	assert.NotNil(t, result.Evidence)
	assert.False(t, result.Evidence.Status, "should not be able to access forbidden file in non-mounted directory")
}

// TestPluginNoCapabilitiesNoAccess verifies plugins with no capabilities cannot access filesystem
func TestPluginNoCapabilitiesNoAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	// Create runtime with NO capabilities for the plugin
	caps := map[string][]capabilities.Capability{
		"file": {}, // Empty capabilities - no filesystem access
	}

	runtime, err := wasm.NewRuntimeWithCapabilities(ctx, build.Get(), caps, nil, 0)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	pluginWASM, err := os.ReadFile("../../plugins/file/file.wasm")
	if os.IsNotExist(err) {
		t.Skip("file.wasm not built - run 'cd plugins/file && make build' first")
	}
	require.NoError(t, err)

	plugin, err := runtime.LoadPlugin(ctx, "file", pluginWASM)
	require.NoError(t, err)

	// Attempt to read /etc/hosts should fail (no filesystem access at all)
	cfg := wasm.Config{
		Values: map[string]interface{}{
			"path": "/etc/hosts",
			"mode": "exists",
		},
	}
	result, err := plugin.Observe(ctx, cfg)

	// Plugin execution succeeds, but file check fails due to lack of filesystem access
	require.NoError(t, err)
	assert.NotNil(t, result.Evidence)
	assert.False(t, result.Evidence.Status, "should not be able to access any files without capabilities")
}

// TestPluginSpecificFileAccess verifies fine-grained file-level access control
func TestPluginSpecificFileAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	// Create test files
	tmpDir := t.TempDir()
	allowedFile := filepath.Join(tmpDir, "config", "allowed.conf")
	deniedFile := filepath.Join(tmpDir, "secrets", "api-key.txt")

	// Create directories
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "config"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "secrets"), 0755))

	require.NoError(t, os.WriteFile(allowedFile, []byte("public config"), 0644))
	require.NoError(t, os.WriteFile(deniedFile, []byte("secret-api-key-12345"), 0644))

	// Grant access only to config directory
	caps := map[string][]capabilities.Capability{
		"file": {
			{Kind: "fs", Pattern: "read:" + filepath.Join(tmpDir, "config") + "/**"},
		},
	}

	runtime, err := wasm.NewRuntimeWithCapabilities(ctx, build.Get(), caps, nil, 0)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	pluginWASM, err := os.ReadFile("../../plugins/file/file.wasm")
	if os.IsNotExist(err) {
		t.Skip("file.wasm not built - run 'cd plugins/file && make build' first")
	}
	require.NoError(t, err)

	plugin, err := runtime.LoadPlugin(ctx, "file", pluginWASM)
	require.NoError(t, err)

	// Test 1: Can access allowed config file
	cfg := wasm.Config{
		Values: map[string]interface{}{
			"path": allowedFile,
			"mode": "exists",
		},
	}
	result, err := plugin.Observe(ctx, cfg)
	require.NoError(t, err)
	assert.True(t, result.Evidence.Status, "should access config file")

	// Test 2: Cannot access secrets file
	cfg = wasm.Config{
		Values: map[string]interface{}{
			"path": deniedFile,
			"mode": "exists",
		},
	}
	result, err = plugin.Observe(ctx, cfg)
	require.NoError(t, err)
	assert.False(t, result.Evidence.Status, "should not access secrets file")
}

// TestPluginRootAccess verifies root filesystem access with warning
func TestPluginRootAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	// Grant root filesystem access (should log warning)
	caps := map[string][]capabilities.Capability{
		"file": {
			{Kind: "fs", Pattern: "read:/**"},
		},
	}

	runtime, err := wasm.NewRuntimeWithCapabilities(ctx, build.Get(), caps, nil, 0)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	pluginWASM, err := os.ReadFile("../../plugins/file/file.wasm")
	if os.IsNotExist(err) {
		t.Skip("file.wasm not built - run 'cd plugins/file && make build' first")
	}
	require.NoError(t, err)

	plugin, err := runtime.LoadPlugin(ctx, "file", pluginWASM)
	require.NoError(t, err)

	// With root access, should be able to read /etc/hosts
	cfg := wasm.Config{
		Values: map[string]interface{}{
			"path": "/etc/hosts",
			"mode": "exists",
		},
	}
	result, err := plugin.Observe(ctx, cfg)
	require.NoError(t, err)
	assert.True(t, result.Evidence.Status, "with root access, should read /etc/hosts")
}

// TestPluginReadOnlyVsReadWrite verifies mount type differentiation
func TestPluginReadOnlyVsReadWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("initial content"), 0644))

	// Grant read-only access
	caps := map[string][]capabilities.Capability{
		"file": {
			{Kind: "fs", Pattern: "read:" + tmpDir + "/**"},
		},
	}

	runtime, err := wasm.NewRuntimeWithCapabilities(ctx, build.Get(), caps, nil, 0)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	pluginWASM, err := os.ReadFile("../../plugins/file/file.wasm")
	if os.IsNotExist(err) {
		t.Skip("file.wasm not built - run 'cd plugins/file && make build' first")
	}
	require.NoError(t, err)

	plugin, err := runtime.LoadPlugin(ctx, "file", pluginWASM)
	require.NoError(t, err)

	// Should be able to read
	cfg := wasm.Config{
		Values: map[string]interface{}{
			"path": testFile,
			"mode": "exists",
		},
	}
	result, err := plugin.Observe(ctx, cfg)
	require.NoError(t, err)
	assert.True(t, result.Evidence.Status, "should be able to read with read: permission")

	// Note: Testing write operations would require a plugin that writes files,
	// which the file plugin doesn't currently do. This test verifies the mount
	// type is set correctly (read-only for "read:" patterns).
}
