package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/reglet-dev/reglet/internal/application/dto"
	"github.com/reglet-dev/reglet/internal/infrastructure/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLockfileGeneration(t *testing.T) {
	// Setup temp workspace
	tmpDir := t.TempDir()
	profilePath := filepath.Join(tmpDir, "profile.yaml")

	// Create a dummy profile
	profileContent := `
profile:
  name: test-profile
  version: 1.0.0
plugins:
  - "reglet/test@1.0"
controls:
  items:
    - id: test-control
      name: Test Control
      observations:
        - plugin: reglet/test
          expect: ["true"] # Dummy
`
	err := os.WriteFile(profilePath, []byte(profileContent), 0o644)
	require.NoError(t, err)

	// Initializes container
	c, err := container.New(container.Options{
		SecurityLevel: "standard",
		TrustPlugins:  true,
		// SystemConfigPath: "/dev/null", // Use defaults
	})
	require.NoError(t, err)

	// Execute check

	// This will invoke the use case, which should trigger lockfile service
	ctx := context.Background()
	req := buildTestRequest(profilePath)

	// We expect execution to fail on engine because plugin "reglet/test" doesn't exist
	// BUT, Lockfile resolution happens BEFORE engine execution.
	// So lockfile SHOULD be created even if execution fails later.
	_, execErr := c.CheckProfileUseCase().Execute(ctx, req)
	if execErr != nil {
		t.Logf("Execution error (ignored if expected): %+v", execErr)
	}

	// Check if lockfile was created
	lockPath := filepath.Join(tmpDir, "reglet.lock")

	// Debug info
	if _, statErr := os.Stat(lockPath); os.IsNotExist(statErr) {
		t.Logf("Lockfile not found at %s", lockPath)
	} else {
		content, _ := os.ReadFile(lockPath)
		t.Logf("Lockfile content:\n%s", string(content))
	}

	exists := false
	if _, err := os.Stat(lockPath); err == nil {
		exists = true
	}
	assert.True(t, exists, "lockfile should be generated")

	// Verify content if exists
	if exists {
		content, _ := os.ReadFile(lockPath)
		assert.Contains(t, string(content), "reglet/test", "lockfile should contain requested plugin")
		assert.Contains(t, string(content), "1.0", "lockfile should contain version")
	}
}

// Helper to build request (mimics CLI builder)
func buildTestRequest(path string) dto.CheckProfileRequest {
	// Since I can't import internal/application/dto easily if it's internal and I am outside
	// Wait, integration tests in Go can import internal packages if they are under the same module root?
	// Yes, "internal" is visible to the module itself.
	// package integration_test is in internal/test/integration which is inside module.
	return dto.CheckProfileRequest{
		ProfilePath: path,
		Metadata: dto.RequestMetadata{
			RequestID: "test-req",
		},
		Execution: dto.ExecutionOptions{
			Parallel: true,
		},
		// Initialize Maps to avoid nil pointer dereference if referenced
		Filters: dto.FilterOptions{},
	}
}
