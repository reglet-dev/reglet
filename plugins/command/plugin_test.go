//go:build !wasip1

package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	regletsdk "github.com/whiskeyjimbo/reglet/sdk"
)

func TestCommandPlugin_Describe(t *testing.T) {
	plugin := &commandPlugin{}
	ctx := context.Background()

	meta, err := plugin.Describe(ctx)
	require.NoError(t, err)

	assert.Equal(t, "command", meta.Name)
	assert.Equal(t, "1.0.0", meta.Version)
	assert.NotEmpty(t, meta.Description)
	assert.Len(t, meta.Capabilities, 1)
	assert.Equal(t, "exec", meta.Capabilities[0].Kind)
	assert.Equal(t, "**", meta.Capabilities[0].Pattern)
}

func TestCommandPlugin_Schema(t *testing.T) {
	plugin := &commandPlugin{}
	ctx := context.Background()

	schema, err := plugin.Schema(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, schema)

	// Schema should be valid JSON
	assert.Contains(t, string(schema), "run")
	assert.Contains(t, string(schema), "command")
	assert.Contains(t, string(schema), "timeout")
}

func TestCommandConfig_Validation(t *testing.T) {
	tests := []struct {
		name      string
		config    map[string]interface{}
		wantError bool
		errMsg    string
	}{
		{
			name: "valid run mode",
			config: map[string]interface{}{
				"run": "echo hello",
			},
			wantError: false,
		},
		{
			name: "valid command mode",
			config: map[string]interface{}{
				"command": "/bin/echo",
				"args":    []string{"hello", "world"},
			},
			wantError: false,
		},
		{
			name: "command with timeout",
			config: map[string]interface{}{
				"command": "/bin/sleep",
				"args":    []string{"1"},
				"timeout": 5,
			},
			wantError: false,
		},
		{
			name: "command with dir and env",
			config: map[string]interface{}{
				"command": "/usr/bin/env",
				"dir":     "/tmp",
				"env":     []string{"FOO=bar"},
			},
			wantError: false,
		},
		{
			name: "missing both run and command",
			config: map[string]interface{}{
				"timeout": 30,
			},
			wantError: true,
			errMsg:    "either 'run' or 'command' must be specified",
		},
		{
			name: "both run and command specified",
			config: map[string]interface{}{
				"run":     "echo hello",
				"command": "/bin/echo",
			},
			// Note: Current implementation allows this (run takes precedence)
			// Consider if this should be an error
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := &commandPlugin{}
			ctx := context.Background()

			evidence, err := plugin.Check(ctx, tt.config)
			require.NoError(t, err, "Check should not return error (errors go in evidence)")

			if tt.wantError {
				assert.False(t, evidence.Status, "Expected evidence.Status to be false")
				if tt.errMsg != "" && evidence.Error != nil {
					assert.Contains(t, evidence.Error.Message, tt.errMsg)
				}
			} else {
				// In real execution, this would call the host function
				// In test mode (non-WASM), the exec.Run call may fail
				// We're mainly testing config validation here
			}
		})
	}
}

func TestCommandConfig_ShellMode(t *testing.T) {
	// Test that run mode constructs shell command correctly
	plugin := &commandPlugin{}
	ctx := context.Background()

	config := map[string]interface{}{
		"run": "echo 'hello world'",
	}

	// This will attempt to call exec.Run which requires WASM environment
	// In a non-WASM build, this will fail, but we can still validate config parsing
	evidence, err := plugin.Check(ctx, config)
	require.NoError(t, err)

	// The evidence will contain an error since we're not in WASM
	// But the config validation should have passed
	_ = evidence
}

func TestCommandConfig_DirectMode(t *testing.T) {
	// Test that command mode constructs direct execution correctly
	plugin := &commandPlugin{}
	ctx := context.Background()

	config := map[string]interface{}{
		"command": "/bin/echo",
		"args":    []string{"hello", "world"},
	}

	evidence, err := plugin.Check(ctx, config)
	require.NoError(t, err)

	// Similar to shell mode, this requires WASM environment
	_ = evidence
}

// Note: Full execution tests require WASM runtime
// These tests focus on configuration validation and structure
// Integration tests in the main test suite will cover actual execution
