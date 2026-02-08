package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestMain runs before all tests in the integration package.
// It ensures the binary and plugins are built once before tests start.
func TestMain(m *testing.M) {
	// Build binary before tests (unless SHORT env var is set)
	// We can't use testing.Short() here because it's not initialized yet
	if os.Getenv("SHORT") == "" {
		if err := ensureBinaryBuilt(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to build binary: %v\n", err)
			os.Exit(1)
		}
	}

	// Run tests
	code := m.Run()
	os.Exit(code)
}

// ensureBinaryBuilt ensures the reglet binary is built before tests run.
func ensureBinaryBuilt() error {
	rootDir, err := findRootDir()
	if err != nil {
		return err
	}

	binPath := filepath.Join(rootDir, "bin", "reglet")

	// Check if binary already exists and is recent
	if info, err := os.Stat(binPath); err == nil {
		// Binary exists - check if it's newer than main.go
		mainGo := filepath.Join(rootDir, "cmd", "reglet", "main.go")
		if mainInfo, err := os.Stat(mainGo); err == nil {
			if info.ModTime().After(mainInfo.ModTime()) {
				// Binary is newer than source, skip build
				return nil
			}
		}
	}

	// Build the binary
	fmt.Println("Building reglet binary for integration tests...")
	cmd := exec.Command("make", "build")
	cmd.Dir = rootDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// findRootDir locates the project root directory.
func findRootDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up from test/integration to find go.mod
	dir := cwd
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find project root (go.mod not found)")
		}
		dir = parent
	}
}

// findPluginDir locates the plugin directory containing built WASM plugins.
// It mirrors the adapter's resolution logic: sibling reglet-plugins/plugins,
// then ./plugins relative to project root.
// Returns "" if no plugin directory is found.
func findPluginDir(t *testing.T) string {
	t.Helper()

	rootDir := findProjectRoot(t)

	// 1. Try sibling reglet-plugins/plugins (workspace layout)
	sibling := filepath.Join(filepath.Dir(rootDir), "reglet-plugins", "plugins")
	if _, err := os.Stat(sibling); err == nil {
		return sibling
	}

	// 2. Try ./plugins relative to project root
	local := filepath.Join(rootDir, "plugins")
	if _, err := os.Stat(local); err == nil {
		return local
	}

	return ""
}

// requirePlugins skips the test if the plugin directory with required WASM
// files is not available (e.g., standalone checkout without built plugins).
func requirePlugins(t *testing.T, plugins ...string) string {
	t.Helper()

	pluginDir := findPluginDir(t)
	if pluginDir == "" {
		t.Skipf("plugin directory not found - run 'make build-plugins' or use workspace with reglet-plugins")
	}

	for _, name := range plugins {
		wasmPath := filepath.Join(pluginDir, name, name+".wasm")
		if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
			t.Skipf("plugin %s not built at %s", name, wasmPath)
		}
	}

	return pluginDir
}
