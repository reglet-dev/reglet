package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInit_NonInteractiveMode verifies non-interactive profile creation.
// T028: Integration test for non-interactive mode
func TestInit_NonInteractiveMode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tests := []struct {
		name       string
		args       []string
		wantErr    bool
		errContain string
		checkFile  bool
	}{
		{
			name:      "creates profile with name and plugins",
			args:      []string{"init", "--name=test-profile", "--plugins=file", "--output=./test.yaml"},
			wantErr:   false,
			checkFile: true,
		},
		{
			name:      "creates profile with multiple plugins",
			args:      []string{"init", "--name=multi", "--plugins=file,http,dns", "--output=./multi.yaml"},
			wantErr:   false,
			checkFile: true,
		},
		{
			name:       "fails with missing name",
			args:       []string{"init", "--plugins=file", "--output=./test.yaml"},
			wantErr:    true,
			errContain: "non-interactive",
		},
		{
			name:       "fails with missing plugins",
			args:       []string{"init", "--name=test", "--output=./test.yaml"},
			wantErr:    true,
			errContain: "non-interactive",
		},
		{
			name:       "fails with invalid plugin",
			args:       []string{"init", "--name=test", "--plugins=invalid", "--output=./test.yaml"},
			wantErr:    true,
			errContain: "unknown plugin",
		},
		{
			name:       "fails with invalid profile name",
			args:       []string{"init", "--name=123invalid", "--plugins=file", "--output=./test.yaml"},
			wantErr:    true,
			errContain: "invalid profile name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Fix output path to use temp directory
			args := make([]string, len(tt.args))
			copy(args, tt.args)
			for i, arg := range args {
				if strings.HasPrefix(arg, "--output=./") {
					args[i] = "--output=" + filepath.Join(tmpDir, arg[10:])
				}
			}

			cmd := exec.Command("go", append([]string{"run", "./cmd/reglet/"}, args...)...)
			cmd.Dir = findProjectRoot(t)
			cmd.Stdin = strings.NewReader("") // Simulate non-interactive

			output, err := cmd.CombinedOutput()

			if tt.wantErr {
				require.Error(t, err, "expected error, output: %s", output)
				if tt.errContain != "" {
					assert.Contains(t, string(output), tt.errContain)
				}
			} else {
				require.NoError(t, err, "unexpected error: %s", output)
				if tt.checkFile {
					// Verify file was created
					for i, arg := range args {
						if strings.HasPrefix(arg, "--output=") {
							outPath := args[i][9:]
							_, err := os.Stat(outPath)
							require.NoError(t, err, "output file should exist: %s", outPath)

							// Verify content
							content, err := os.ReadFile(outPath)
							require.NoError(t, err)
							assert.Contains(t, string(content), "profile:")
							assert.Contains(t, string(content), "controls:")
						}
					}
				}
			}
		})
	}
}

// TestInit_ProfileNameValidation verifies profile name validation.
// T015: Integration test for profile name validation
func TestInit_ProfileNameValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tests := []struct {
		name    string
		profile string
		wantErr bool
	}{
		{"valid lowercase", "myprofile", false},
		{"valid with hyphen", "my-profile", false},
		{"valid with underscore", "my_profile", false},
		{"valid with numbers", "profile123", false},
		{"starts with number", "1profile", true},
		{"has spaces", "my profile", true},
		{"has special chars", "my@profile", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			outPath := filepath.Join(tmpDir, "test.yaml")

			cmd := exec.Command("go", "run", "./cmd/reglet/",
				"init",
				"--name="+tt.profile,
				"--plugins=file",
				"--output="+outPath)
			cmd.Dir = findProjectRoot(t)
			cmd.Stdin = strings.NewReader("")

			output, err := cmd.CombinedOutput()

			if tt.wantErr {
				require.Error(t, err, "expected error for profile name %q, output: %s", tt.profile, output)
				assert.Contains(t, string(output), "invalid profile name")
			} else {
				require.NoError(t, err, "unexpected error for profile name %q: %s", tt.profile, output)
			}
		})
	}
}

// TestInit_ConfigGeneration verifies --with-config flag behavior.
// T037: Integration test for config generation
func TestInit_ConfigGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	profilePath := filepath.Join(tmpDir, "profile.yaml")

	// Preserve Go module cache path before changing HOME
	goModCache := os.Getenv("GOMODCACHE")
	if goModCache == "" {
		// Default GOMODCACHE is $HOME/go/pkg/mod
		home, _ := os.UserHomeDir()
		goModCache = filepath.Join(home, "go", "pkg", "mod")
	}

	// Set HOME to temp directory to avoid writing to real home
	t.Setenv("HOME", tmpDir)
	// Preserve GOMODCACHE so go run works
	t.Setenv("GOMODCACHE", goModCache)

	cmd := exec.Command("go", "run", "./cmd/reglet/",
		"init",
		"--name=test-config",
		"--plugins=file,http",
		"--output="+profilePath,
		"--with-config")
	cmd.Dir = findProjectRoot(t)
	cmd.Stdin = strings.NewReader("")

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "init with config failed: %s", output)

	// Verify profile created
	_, err = os.Stat(profilePath)
	require.NoError(t, err, "profile should exist")

	// Verify config created
	configPath := filepath.Join(tmpDir, ".reglet", "config.yaml")
	_, err = os.Stat(configPath)
	require.NoError(t, err, "config should exist at %s", configPath)

	// Verify config content
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "capabilities:")
	assert.Contains(t, string(content), "kind: fs")
	assert.Contains(t, string(content), "kind: network")
}

// TestInit_CustomOutputPath verifies --output flag behavior.
// T046: Integration test for custom output path
func TestInit_CustomOutputPath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// Create nested directory path
	outPath := filepath.Join(tmpDir, "nested", "path", "custom.yaml")

	cmd := exec.Command("go", "run", "./cmd/reglet/",
		"init",
		"--name=custom-path",
		"--plugins=file",
		"--output="+outPath)
	cmd.Dir = findProjectRoot(t)
	cmd.Stdin = strings.NewReader("")

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "init failed: %s", output)

	// Verify file created at custom path
	_, err = os.Stat(outPath)
	require.NoError(t, err, "file should exist at custom path: %s", outPath)
}

// TestInit_ParentDirectoryCreation verifies parent directory auto-creation.
// T047: Integration test for parent directory creation
func TestInit_ParentDirectoryCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// Deep nested path that doesn't exist
	outPath := filepath.Join(tmpDir, "a", "b", "c", "d", "profile.yaml")

	cmd := exec.Command("go", "run", "./cmd/reglet/",
		"init",
		"--name=nested",
		"--plugins=file",
		"--output="+outPath)
	cmd.Dir = findProjectRoot(t)
	cmd.Stdin = strings.NewReader("")

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "init failed: %s", output)

	// Verify parent directories created
	dir := filepath.Dir(outPath)
	info, err := os.Stat(dir)
	require.NoError(t, err, "parent directory should exist")
	assert.True(t, info.IsDir())

	// Verify file created
	_, err = os.Stat(outPath)
	require.NoError(t, err, "file should exist")
}

// TestInit_FileExistsError verifies error when file exists without --force.
// T048: Integration test for file exists error
func TestInit_FileExistsError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "existing.yaml")

	// Create existing file
	err := os.WriteFile(outPath, []byte("existing: content"), 0o644)
	require.NoError(t, err)

	// Try to overwrite without --force
	cmd := exec.Command("go", "run", "./cmd/reglet/",
		"init",
		"--name=test",
		"--plugins=file",
		"--output="+outPath)
	cmd.Dir = findProjectRoot(t)
	cmd.Stdin = strings.NewReader("")

	output, err := cmd.CombinedOutput()
	require.Error(t, err, "should fail when file exists, output: %s", output)
	assert.Contains(t, string(output), "file exists")
	assert.Contains(t, string(output), "--force")
}

// TestInit_ForceOverwrite verifies --force flag overwrites existing files.
func TestInit_ForceOverwrite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "existing.yaml")

	// Create existing file
	err := os.WriteFile(outPath, []byte("old: content"), 0o644)
	require.NoError(t, err)

	// Overwrite with --force
	cmd := exec.Command("go", "run", "./cmd/reglet/",
		"init",
		"--name=new-profile",
		"--plugins=http",
		"--output="+outPath,
		"--force")
	cmd.Dir = findProjectRoot(t)
	cmd.Stdin = strings.NewReader("")

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "init with force failed: %s", output)

	// Verify content was overwritten
	content, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "new-profile")
	assert.Contains(t, string(content), "http")
	assert.NotContains(t, string(content), "old: content")
}

// TestInit_AllPlugins verifies all 6 plugins can be included.
func TestInit_AllPlugins(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "all-plugins.yaml")

	cmd := exec.Command("go", "run", "./cmd/reglet/",
		"init",
		"--name=all-plugins",
		"--plugins=file,http,dns,tcp,command,smtp",
		"--output="+outPath)
	cmd.Dir = findProjectRoot(t)
	cmd.Stdin = strings.NewReader("")

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "init failed: %s", output)

	// Verify all plugins in output
	content, err := os.ReadFile(outPath)
	require.NoError(t, err)

	expectedPlugins := []string{"file", "http", "dns", "tcp", "command", "smtp"}
	for _, plugin := range expectedPlugins {
		assert.Contains(t, string(content), plugin)
	}

	expectedControls := []string{
		"file-exists-check",
		"http-health-check",
		"dns-resolution-check",
		"tcp-port-check",
		"command-output-check",
		"smtp-connection-check",
	}
	for _, control := range expectedControls {
		assert.Contains(t, string(content), control)
	}
}

// findProjectRoot locates the project root from the test directory.
func findProjectRoot(t *testing.T) string {
	t.Helper()

	cwd, err := os.Getwd()
	require.NoError(t, err)

	dir := cwd
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (go.mod not found)")
		}
		dir = parent
	}
}
