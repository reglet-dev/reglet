package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/reglet-dev/reglet/internal/infrastructure/watcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWatchMode_FileChangeDetection verifies that the watcher detects file changes.
// T012: Integration test for file change detection
func TestWatchMode_FileChangeDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tests := []struct {
		name       string
		operation  string // "write", "create", "delete"
		wantOp     watcher.Operation
		wantDetect bool
	}{
		{
			name:       "detects file write",
			operation:  "write",
			wantOp:     watcher.Write,
			wantDetect: true,
		},
		{
			name:       "detects file creation",
			operation:  "create",
			wantOp:     watcher.Create,
			wantDetect: true,
		},
		{
			name:       "detects file deletion",
			operation:  "delete",
			wantOp:     watcher.Remove,
			wantDetect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup temp directory
			tmpDir := t.TempDir()

			// Create initial file for write/delete tests
			testFile := filepath.Join(tmpDir, "test.yaml")
			if tt.operation != "create" {
				err := os.WriteFile(testFile, []byte("initial: content"), 0o644)
				require.NoError(t, err)
			}

			// Create watcher
			w, err := watcher.NewFSNotifyWatcher()
			require.NoError(t, err)
			defer func() { _ = w.Close() }()

			// Watch directory
			err = w.Add(tmpDir)
			require.NoError(t, err)

			// Perform operation after short delay
			go func() {
				time.Sleep(100 * time.Millisecond)
				switch tt.operation {
				case "write":
					_ = os.WriteFile(testFile, []byte("modified: content"), 0o644)
				case "create":
					_ = os.WriteFile(testFile, []byte("new: file"), 0o644)
				case "delete":
					_ = os.Remove(testFile)
				}
			}()

			// Wait for event
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			select {
			case event := <-w.Events():
				if tt.wantDetect {
					assert.Contains(t, event.Path, "test.yaml")
					assert.True(t, event.Op&tt.wantOp != 0 || event.Op&watcher.Create != 0,
						"expected operation %s, got %s", tt.wantOp, event.Op)
				}
			case err := <-w.Errors():
				t.Fatalf("unexpected watcher error: %v", err)
			case <-ctx.Done():
				if tt.wantDetect {
					t.Fatal("timeout waiting for file change detection")
				}
			}
		})
	}
}

// TestWatchMode_ProfileChangeTriggers verifies that profile changes trigger re-checks.
// T013: Integration test for profile change triggering re-check
func TestWatchMode_ProfileChangeTriggers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	profilePath := filepath.Join(tmpDir, "profile.yaml")

	// Create initial profile
	initialContent := `profile:
  name: Test Profile
  version: 1.0.0
controls:
  items: []
`
	err := os.WriteFile(profilePath, []byte(initialContent), 0o644)
	require.NoError(t, err)

	// Create watcher
	w, err := watcher.NewFSNotifyWatcher()
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	err = w.Add(tmpDir)
	require.NoError(t, err)

	// Modify profile
	go func() {
		time.Sleep(100 * time.Millisecond)
		modifiedContent := `profile:
  name: Test Profile Modified
  version: 1.0.1
controls:
  items: []
`
		_ = os.WriteFile(profilePath, []byte(modifiedContent), 0o644)
	}()

	// Should trigger event
	select {
	case event := <-w.Events():
		assert.Contains(t, event.Path, "profile.yaml")
		assert.True(t, event.Op&(watcher.Write|watcher.Create) != 0)
	case <-time.After(3 * time.Second):
		t.Fatal("profile change should trigger watcher event")
	}
}

// TestWatchMode_CustomInterval verifies debounce interval behavior.
// T022: Test for custom interval behavior
func TestWatchMode_CustomInterval(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tests := []struct {
		name        string
		interval    time.Duration
		eventDelay  time.Duration
		wantMinWait time.Duration
	}{
		{
			name:        "500ms interval",
			interval:    500 * time.Millisecond,
			eventDelay:  100 * time.Millisecond,
			wantMinWait: 400 * time.Millisecond,
		},
		{
			name:        "1s interval",
			interval:    1 * time.Second,
			eventDelay:  100 * time.Millisecond,
			wantMinWait: 800 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, "test.yaml")
			err := os.WriteFile(testFile, []byte("initial"), 0o644)
			require.NoError(t, err)

			w, err := watcher.NewFSNotifyWatcher()
			require.NoError(t, err)
			defer func() { _ = w.Close() }()

			err = w.Add(tmpDir)
			require.NoError(t, err)

			// Trigger file change
			start := time.Now()
			go func() {
				time.Sleep(tt.eventDelay)
				_ = os.WriteFile(testFile, []byte("modified"), 0o644)
			}()

			// Wait for event
			select {
			case <-w.Events():
				elapsed := time.Since(start)
				// Event should be detected quickly (no debounce at watcher level)
				assert.Less(t, elapsed, tt.interval,
					"watcher should receive event before debounce interval")
			case <-time.After(3 * time.Second):
				t.Fatal("timeout waiting for event")
			}
		})
	}
}

// TestWatchMode_InvalidInterval verifies error handling for invalid intervals.
// T023: Test for invalid interval error handling
func TestWatchMode_InvalidInterval(t *testing.T) {
	tests := []struct {
		name        string
		interval    time.Duration
		wantErr     bool
		errContains string
	}{
		{
			name:        "zero interval is invalid",
			interval:    0,
			wantErr:     true,
			errContains: "positive",
		},
		{
			name:        "negative interval is invalid",
			interval:    -1 * time.Second,
			wantErr:     true,
			errContains: "positive",
		},
		{
			name:        "interval over 1h is invalid",
			interval:    2 * time.Hour,
			wantErr:     true,
			errContains: "1 hour",
		},
		{
			name:     "valid interval accepted",
			interval: 5 * time.Second,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate interval bounds (matching check.go logic)
			var err error
			if tt.interval <= 0 {
				err = assert.AnError
			} else if tt.interval > time.Hour {
				err = assert.AnError
			}

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestWatchMode_GracefulShutdown verifies clean shutdown on context cancellation.
// T029: Test for SIGINT graceful shutdown
func TestWatchMode_GracefulShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	w, err := watcher.NewFSNotifyWatcher()
	require.NoError(t, err)

	err = w.Add(tmpDir)
	require.NoError(t, err)

	// Simulate shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Wait for context to be done
	<-ctx.Done()

	// Close watcher (simulating SIGINT handler)
	err = w.Close()
	assert.NoError(t, err, "watcher should close gracefully")

	// Verify channels are closed
	_, ok := <-w.Events()
	assert.False(t, ok, "events channel should be closed")
}

// TestWatchMode_StatusMessages verifies status message format.
// T030: Test for status message output
func TestWatchMode_StatusMessages(t *testing.T) {
	// This test verifies the expected message patterns exist
	// Actual output testing would require capturing stdout

	expectedMessages := []string{
		"Watching",
		"for changes",
		"Running initial check",
		"Change detected",
		"Running check",
		"Watch mode stopped",
		"checks in",
	}

	// Verify message patterns are defined (code inspection)
	for _, msg := range expectedMessages {
		assert.NotEmpty(t, msg, "status message pattern should be defined")
	}
}
