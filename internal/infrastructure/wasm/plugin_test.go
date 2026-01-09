package wasm

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/reglet-dev/reglet/internal/domain/capabilities"
	"github.com/reglet-dev/reglet/internal/infrastructure/build"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPlugin_Observe_Concurrent tests that multiple goroutines can call
// Observe() concurrently without data races or cross-contamination
func TestPlugin_Observe_Concurrent(t *testing.T) {
	ctx := context.Background()

	// Grant file plugin access to current directory for temp files
	caps := map[string][]capabilities.Capability{
		"file": {
			{Kind: "fs", Pattern: "read:/**"},
		},
	}

	runtime, err := NewRuntimeWithCapabilities(ctx, build.Get(), caps, nil, 0)
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
	results := make([]*PluginObservationResult, numFiles)

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

	// Grant file plugin access to current directory for temp files
	caps := map[string][]capabilities.Capability{
		"file": {
			{Kind: "fs", Pattern: "read:/**"},
		},
	}

	runtime, err := NewRuntimeWithCapabilities(ctx, build.Get(), caps, nil, 0)
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

// TestExtractMountPath tests the extractMountPath function with various patterns
func TestExtractMountPath(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		expected string
	}{
		{
			name:     "specific file",
			pattern:  "/etc/ssh/sshd_config",
			expected: "/etc/ssh",
		},
		{
			name:     "directory with wildcard",
			pattern:  "/var/log/**",
			expected: "/var/log",
		},
		{
			name:     "root pattern",
			pattern:  "/**",
			expected: "/",
		},
		{
			name:     "read operation prefix",
			pattern:  "read:/etc/hosts",
			expected: "/etc",
		},
		{
			name:     "write operation prefix",
			pattern:  "write:/var/log/app.log",
			expected: "/var/log",
		},
		{
			name:     "single slash",
			pattern:  "/",
			expected: "/",
		},
		{
			name:     "directory without trailing slash",
			pattern:  "/etc",
			expected: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMountPath(tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestExtractMountPath_RelativePaths tests the security fix for relative paths
// SECURITY: Relative paths should NEVER mount root (/) - they should mount CWD
func TestExtractMountPath_RelativePaths(t *testing.T) {
	// Get current working directory for comparison
	cwd, err := os.Getwd()
	require.NoError(t, err, "test setup: failed to get current directory")

	tests := []struct {
		name     string
		pattern  string
		expected string
	}{
		{
			name:     "relative file without path",
			pattern:  "foo.txt",
			expected: cwd, // Should mount CWD, NOT root!
		},
		{
			name:     "relative file with read prefix",
			pattern:  "read:config.yaml",
			expected: cwd, // Should mount CWD, NOT root!
		},
		{
			name:     "relative file with write prefix",
			pattern:  "write:output.log",
			expected: cwd, // Should mount CWD, NOT root!
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMountPath(tt.pattern)
			assert.Equal(t, tt.expected, result,
				"SECURITY: relative path '%s' must mount CWD (%s), NOT root (/)",
				tt.pattern, cwd)
		})
	}
}

// TestPlugin_ExtractFilesystemMounts tests the extractFilesystemMounts method
func TestPlugin_ExtractFilesystemMounts(t *testing.T) {
	tests := []struct {
		name         string
		capabilities []capabilities.Capability
		expected     []fsMount
	}{
		{
			name: "read-only file access",
			capabilities: []capabilities.Capability{
				{Kind: "fs", Pattern: "read:/etc/hosts"},
			},
			expected: []fsMount{
				{hostPath: "/etc", guestPath: "/etc", readOnly: true},
			},
		},
		{
			name: "read-write directory access",
			capabilities: []capabilities.Capability{
				{Kind: "fs", Pattern: "write:/var/log/**"},
			},
			expected: []fsMount{
				{hostPath: "/var/log", guestPath: "/var/log", readOnly: false},
			},
		},
		{
			name: "mixed permissions",
			capabilities: []capabilities.Capability{
				{Kind: "fs", Pattern: "read:/etc/hosts"},
				{Kind: "fs", Pattern: "write:/var/log/app.log"},
			},
			expected: []fsMount{
				{hostPath: "/etc", guestPath: "/etc", readOnly: true},
				{hostPath: "/var/log", guestPath: "/var/log", readOnly: false},
			},
		},
		{
			name: "no filesystem capabilities",
			capabilities: []capabilities.Capability{
				{Kind: "network", Pattern: "outbound:443"},
			},
			expected: []fsMount{},
		},
		{
			name: "root access",
			capabilities: []capabilities.Capability{
				{Kind: "fs", Pattern: "read:/**"},
			},
			expected: []fsMount{
				{hostPath: "/", guestPath: "/", readOnly: true},
			},
		},
		{
			name: "duplicate mounts filtered",
			capabilities: []capabilities.Capability{
				{Kind: "fs", Pattern: "read:/etc/hosts"},
				{Kind: "fs", Pattern: "read:/etc/passwd"},
			},
			expected: []fsMount{
				{hostPath: "/etc", guestPath: "/etc", readOnly: true},
			},
		},
		{
			name: "overlapping mounts not deduplicated",
			capabilities: []capabilities.Capability{
				{Kind: "fs", Pattern: "read:/etc/**"},
				{Kind: "fs", Pattern: "read:/etc/ssh/**"},
			},
			expected: []fsMount{
				{hostPath: "/etc", guestPath: "/etc", readOnly: true},
				{hostPath: "/etc/ssh", guestPath: "/etc/ssh", readOnly: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := &Plugin{
				name:         "test-plugin",
				capabilities: tt.capabilities,
			}

			mounts := plugin.extractFilesystemMounts()
			assert.Equal(t, len(tt.expected), len(mounts), "mount count mismatch")

			// Compare mounts (order may vary)
			for _, expectedMount := range tt.expected {
				found := false
				for _, actualMount := range mounts {
					if actualMount.hostPath == expectedMount.hostPath &&
						actualMount.guestPath == expectedMount.guestPath &&
						actualMount.readOnly == expectedMount.readOnly {
						found = true
						break
					}
				}
				assert.True(t, found, "expected mount not found: %+v", expectedMount)
			}
		})
	}
}
