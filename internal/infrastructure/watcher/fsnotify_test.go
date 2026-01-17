package watcher_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/reglet-dev/reglet/internal/infrastructure/watcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFSNotifyWatcher_NewAndClose(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "creates watcher successfully",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w, err := watcher.NewFSNotifyWatcher()
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, w)

			// Verify channels are available
			assert.NotNil(t, w.Events())
			assert.NotNil(t, w.Errors())

			// Close should succeed
			err = w.Close()
			assert.NoError(t, err)
		})
	}
}

func TestFSNotifyWatcher_AddRemove(t *testing.T) {
	t.Parallel()

	// Create temp directory
	tmpDir := t.TempDir()

	tests := []struct {
		name       string
		addPath    string
		removePath string
		wantAddErr bool
		wantRemErr bool
	}{
		{
			name:       "add and remove valid directory",
			addPath:    tmpDir,
			removePath: tmpDir,
			wantAddErr: false,
			wantRemErr: false,
		},
		{
			name:       "add non-existent path",
			addPath:    filepath.Join(tmpDir, "nonexistent"),
			wantAddErr: true,
		},
		{
			name:       "remove non-watched path",
			addPath:    tmpDir,
			removePath: filepath.Join(tmpDir, "other"),
			wantAddErr: false,
			wantRemErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w, err := watcher.NewFSNotifyWatcher()
			require.NoError(t, err)
			defer func() { _ = w.Close() }()

			// Test Add
			err = w.Add(tt.addPath)
			if tt.wantAddErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			// Test Remove
			if tt.removePath != "" {
				err = w.Remove(tt.removePath)
				if tt.wantRemErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			}
		})
	}
}

func TestFSNotifyWatcher_DetectsFileWrite(t *testing.T) {
	t.Parallel()

	// Create temp directory and file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("initial"), 0o644)
	require.NoError(t, err)

	// Create watcher
	w, err := watcher.NewFSNotifyWatcher()
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	// Watch the temp directory
	err = w.Add(tmpDir)
	require.NoError(t, err)

	// Modify file
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = os.WriteFile(testFile, []byte("modified"), 0o644)
	}()

	// Wait for event
	select {
	case event := <-w.Events():
		assert.Contains(t, event.Path, "test.txt")
		// Write may appear as Create or Write depending on OS
		assert.True(t, event.Op&watcher.Write != 0 || event.Op&watcher.Create != 0,
			"expected Write or Create operation, got %s", event.Op)
	case err := <-w.Errors():
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for file event")
	}
}

func TestFSNotifyWatcher_DetectsFileCreate(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	w, err := watcher.NewFSNotifyWatcher()
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	err = w.Add(tmpDir)
	require.NoError(t, err)

	// Create new file
	go func() {
		time.Sleep(50 * time.Millisecond)
		newFile := filepath.Join(tmpDir, "newfile.txt")
		_ = os.WriteFile(newFile, []byte("new"), 0o644)
	}()

	select {
	case event := <-w.Events():
		assert.Contains(t, event.Path, "newfile.txt")
		assert.True(t, event.Op&watcher.Create != 0, "expected Create operation")
	case err := <-w.Errors():
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for create event")
	}
}

func TestFSNotifyWatcher_DetectsFileRemove(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "todelete.txt")
	err := os.WriteFile(testFile, []byte("delete me"), 0o644)
	require.NoError(t, err)

	w, err := watcher.NewFSNotifyWatcher()
	require.NoError(t, err)
	defer func() { _ = w.Close() }()

	err = w.Add(tmpDir)
	require.NoError(t, err)

	// Delete file
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = os.Remove(testFile)
	}()

	select {
	case event := <-w.Events():
		assert.Contains(t, event.Path, "todelete.txt")
		assert.True(t, event.Op&watcher.Remove != 0, "expected Remove operation")
	case err := <-w.Errors():
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for remove event")
	}
}

func TestOperation_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		op   watcher.Operation
		want string
	}{
		{watcher.Create, "CREATE"},
		{watcher.Write, "WRITE"},
		{watcher.Remove, "REMOVE"},
		{watcher.Rename, "RENAME"},
		{watcher.Chmod, "CHMOD"},
		{watcher.Operation(0), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.op.String())
		})
	}
}

// Benchmarks for watcher performance
// T044: Benchmark for watcher event handling

func BenchmarkFSNotifyWatcher_Create(b *testing.B) {
	for i := 0; i < b.N; i++ {
		w, err := watcher.NewFSNotifyWatcher()
		if err != nil {
			b.Fatal(err)
		}
		_ = w.Close()
	}
}

func BenchmarkFSNotifyWatcher_AddRemove(b *testing.B) {
	tmpDir := b.TempDir()

	w, err := watcher.NewFSNotifyWatcher()
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = w.Close() }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = w.Add(tmpDir)
		_ = w.Remove(tmpDir)
	}
}

func BenchmarkFSNotifyWatcher_EventReceive(b *testing.B) {
	tmpDir := b.TempDir()
	testFile := filepath.Join(tmpDir, "bench.txt")
	_ = os.WriteFile(testFile, []byte("initial"), 0o644)

	w, err := watcher.NewFSNotifyWatcher()
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = w.Close() }()

	_ = w.Add(tmpDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Write to trigger event
		_ = os.WriteFile(testFile, []byte("modified"), 0o644)

		// Wait for event with timeout
		select {
		case <-w.Events():
			// Event received
		case <-time.After(100 * time.Millisecond):
			// Timeout (acceptable for benchmark)
		}
	}
}

func BenchmarkOperation_String(b *testing.B) {
	ops := []watcher.Operation{
		watcher.Create,
		watcher.Write,
		watcher.Remove,
		watcher.Rename,
		watcher.Chmod,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ops[i%len(ops)].String()
	}
}
