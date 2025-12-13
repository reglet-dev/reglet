package wasm

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/whiskeyjimbo/reglet/internal/domain/capabilities"
)

// TestCommandPlugin_Integration tests the command plugin end-to-end with WASM runtime.
func TestCommandPlugin_Integration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Grant exec capabilities for testing
	grantedCaps := map[string][]capabilities.Capability{
		"command": {
			{Kind: "exec", Pattern: "/bin/echo"},
			{Kind: "exec", Pattern: "/bin/sh"},
			{Kind: "exec", Pattern: "/usr/bin/env"},
		},
	}

	runtime, err := NewRuntimeWithCapabilities(ctx, grantedCaps)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	// Find command plugin
	pluginPath := findCommandPlugin(t)
	wasmBytes, err := os.ReadFile(pluginPath)
	require.NoError(t, err, "Failed to read command.wasm")

	// Load plugin
	plugin, err := runtime.LoadPlugin(ctx, "command", wasmBytes)
	require.NoError(t, err)

	t.Run("DirectExecution_Echo", func(t *testing.T) {
		config := Config{
			Values: map[string]interface{}{
				"command": "/bin/echo",
				"args":    []string{"hello", "world"},
			},
		}

		result, err := plugin.Observe(ctx, config)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Nil(t, result.Error, "Should not have error")
		require.NotNil(t, result.Evidence)

		assert.True(t, result.Evidence.Status, "Command should succeed")

		stdout, _ := result.Evidence.Data["stdout"].(string)
		assert.Equal(t, "hello world", stdout)
		exitCode, _ := result.Evidence.Data["exit_code"].(float64)
		assert.Equal(t, float64(0), exitCode)
	})

	t.Run("ShellExecution_Simple", func(t *testing.T) {
		config := Config{
			Values: map[string]interface{}{
				"run": "echo 'test output'",
			},
		}

		result, err := plugin.Observe(ctx, config)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Nil(t, result.Error)
		require.NotNil(t, result.Evidence)

		assert.True(t, result.Evidence.Status, "Shell command should succeed")

		stdout, _ := result.Evidence.Data["stdout"].(string)
		assert.Equal(t, "test output", stdout)
	})

	t.Run("ExitCode_NonZero", func(t *testing.T) {
		config := Config{
			Values: map[string]interface{}{
				"run": "exit 42",
			},
		}

		result, err := plugin.Observe(ctx, config)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Nil(t, result.Error, "Should not have error for non-zero exit code")
		require.NotNil(t, result.Evidence)

		// Exit code 42 should result in status=false
		assert.False(t, result.Evidence.Status, "Non-zero exit should set status=false")

		exitCode, ok := result.Evidence.Data["exit_code"].(float64)
		require.True(t, ok, "exit_code field should be present")
		assert.Equal(t, float64(42), exitCode)
	})

	t.Run("Stderr_Capture", func(t *testing.T) {
		config := Config{
			Values: map[string]interface{}{
				"run": "echo 'error message' >&2",
			},
		}

		result, err := plugin.Observe(ctx, config)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Nil(t, result.Error)
		require.NotNil(t, result.Evidence)

		stderr, _ := result.Evidence.Data["stderr"].(string)
		assert.Equal(t, "error message", stderr)
	})

	t.Run("Environment_Variables", func(t *testing.T) {
		config := Config{
			Values: map[string]interface{}{
				"command": "/usr/bin/env",
				"env":     []string{"TEST_VAR=test_value"},
			},
		}

		result, err := plugin.Observe(ctx, config)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Nil(t, result.Error)
		require.NotNil(t, result.Evidence)

		stdout, _ := result.Evidence.Data["stdout"].(string)
		assert.Contains(t, stdout, "TEST_VAR=test_value")
	})
}

// TestCommandPlugin_Capabilities tests capability enforcement.
func TestCommandPlugin_Capabilities(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create runtime with NO exec capabilities
	runtime, err := NewRuntime(ctx)
	require.NoError(t, err)
	defer runtime.Close(ctx)

	pluginPath := findCommandPlugin(t)
	wasmBytes, err := os.ReadFile(pluginPath)
	require.NoError(t, err)

	plugin, err := runtime.LoadPlugin(ctx, "command", wasmBytes)
	require.NoError(t, err)

	t.Run("Denied_NoCapability", func(t *testing.T) {
		config := Config{
			Values: map[string]interface{}{
				"command": "/bin/echo",
				"args":    []string{"test"},
			},
		}

		result, err := plugin.Observe(ctx, config)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Should fail due to missing capability
		// Capability errors are returned in Evidence.Error
		require.NotNil(t, result.Evidence)
		assert.False(t, result.Evidence.Status, "Should fail without capability")
		require.NotNil(t, result.Evidence.Error, "Evidence.Error should be set for capability error")
		assert.Contains(t, result.Evidence.Error.Message, "permission denied")
	})

	t.Run("Denied_ShellExecution", func(t *testing.T) {
		config := Config{
			Values: map[string]interface{}{
				"run": "echo test",
			},
		}

		result, err := plugin.Observe(ctx, config)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Should fail - requires exec:/bin/sh capability
		// Capability errors are returned in Evidence.Error
		require.NotNil(t, result.Evidence)
		assert.False(t, result.Evidence.Status, "Should fail without shell capability")
		require.NotNil(t, result.Evidence.Error, "Evidence.Error should be set for capability error")
		assert.Contains(t, result.Evidence.Error.Message, "shell execution requires")
	})
}

// findCommandPlugin locates the compiled command.wasm file.
func findCommandPlugin(t *testing.T) string {
	t.Helper()

	// Try common locations
	locations := []string{
		"../../plugins/command/command.wasm",
		"plugins/command/command.wasm",
		"./plugins/command/command.wasm",
	}

	for _, loc := range locations {
		absPath, err := filepath.Abs(loc)
		if err != nil {
			continue
		}
		if _, err := os.Stat(absPath); err == nil {
			return absPath
		}
	}

	t.Skip("command.wasm not found - run 'make' in plugins/command/ first")
	return ""
}
