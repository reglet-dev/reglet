package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadProfileFromReader_Valid(t *testing.T) {
	yaml := `
profile:
  name: Test Profile
  version: 1.0.0
controls:
  items:
    - id: ctrl-001
      name: Test Control
      observations:
        - plugin: http
          config:
            url: http://example.com
`
	loader := NewProfileLoader()
	profile, err := loader.LoadProfileFromReader(strings.NewReader(yaml))

	require.NoError(t, err)
	assert.Equal(t, "Test Profile", profile.Metadata.Name)
	assert.Equal(t, "1.0.0", profile.Metadata.Version)
	assert.Len(t, profile.Controls.Items, 1)
	assert.Equal(t, "ctrl-001", profile.Controls.Items[0].ID)
}

func TestLoadProfileFromReader_InvalidYAML(t *testing.T) {
	yaml := `invalid yaml: [[[`

	loader := NewProfileLoader()
	_, err := loader.LoadProfileFromReader(strings.NewReader(yaml))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode")
}

func TestLoadProfileFromReader_LoadsRawProfile(t *testing.T) {
	// ProfileLoader now just loads the raw profile without validation
	// The application layer is responsible for compiling (validating + applying defaults)
	yaml := `
profile:
  name: ""
  version: 1.0.0
controls:
  items: []
`
	loader := NewProfileLoader()
	profile, err := loader.LoadProfileFromReader(strings.NewReader(yaml))

	// Should successfully load even with invalid data
	// Validation happens in ProfileCompiler, not ProfileLoader
	require.NoError(t, err)
	assert.NotNil(t, profile)
	assert.Empty(t, profile.Metadata.Name) // Empty name is loaded as-is
}

func TestLoadProfileFromReader_LoadsDefaultsWithoutApplying(t *testing.T) {
	// ProfileLoader now just loads the raw profile
	// Defaults are NOT applied by the loader - that's done by ProfileCompiler
	yaml := `
profile:
  name: Test
  version: 1.0.0
controls:
  defaults:
    severity: high
    owner: platform
  items:
    - id: ctrl-001
      name: Test
      observations:
        - plugin: http
          config:
            url: http://example.com
`
	loader := NewProfileLoader()
	profile, err := loader.LoadProfileFromReader(strings.NewReader(yaml))

	require.NoError(t, err)
	// Defaults are loaded but NOT applied to controls
	assert.NotNil(t, profile.Controls.Defaults)
	assert.Equal(t, "high", profile.Controls.Defaults.Severity)
	assert.Equal(t, "platform", profile.Controls.Defaults.Owner)

	// Control should NOT have defaults applied yet
	assert.Empty(t, profile.Controls.Items[0].Severity)
	assert.Empty(t, profile.Controls.Items[0].Owner)
}

// ===== INHERITANCE TESTS =====

func TestLoadProfile_WithSingleExtends(t *testing.T) {
	t.Parallel()

	// Create temp directory for test files
	tmpDir, err := os.MkdirTemp("", "profile-inheritance-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create base profile
	baseContent := `
profile:
  name: base-security
  version: 1.0.0
plugins:
  - reglet/file@1.0
controls:
  defaults:
    severity: high
  items:
    - id: ssh-config
      name: SSH Configuration
      observations:
        - plugin: file
          config:
            path: /etc/ssh/sshd_config
`
	basePath := filepath.Join(tmpDir, "base.yaml")
	require.NoError(t, os.WriteFile(basePath, []byte(baseContent), 0o644))

	// Create child profile that extends base
	childContent := `
profile:
  name: production
  version: 2.0.0
extends:
  - base.yaml
plugins:
  - reglet/http@1.0
controls:
  defaults:
    tags:
      - production
  items:
    - id: api-health
      name: API Health Check
      observations:
        - plugin: http
          config:
            url: https://api.example.com/health
`
	childPath := filepath.Join(tmpDir, "production.yaml")
	require.NoError(t, os.WriteFile(childPath, []byte(childContent), 0o644))

	// Load the child profile
	loader := NewProfileLoader()
	profile, err := loader.LoadProfile(childPath)

	require.NoError(t, err)
	assert.Equal(t, "production", profile.Metadata.Name)
	assert.Equal(t, "2.0.0", profile.Metadata.Version)

	// Plugins should be merged and deduplicated
	assert.Contains(t, profile.Plugins, "reglet/file@1.0")
	assert.Contains(t, profile.Plugins, "reglet/http@1.0")

	// Controls should include both base and child
	assert.Len(t, profile.Controls.Items, 2)

	// Find controls by ID
	controlIDs := make(map[string]bool)
	for _, ctrl := range profile.Controls.Items {
		controlIDs[ctrl.ID] = true
	}
	assert.True(t, controlIDs["ssh-config"], "Inherited control should be present")
	assert.True(t, controlIDs["api-health"], "Child control should be present")

	// Extends should NOT be propagated after merge
	assert.Nil(t, profile.Extends)
}

func TestLoadProfile_WithMultipleExtends(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "profile-multi-extends-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create first parent
	parent1Content := `
profile:
  name: security-baseline
  version: 1.0.0
vars:
  key: parent1_value
controls:
  items:
    - id: ctrl-1
      name: From Parent1
      observations:
        - plugin: file
`
	parent1Path := filepath.Join(tmpDir, "security.yaml")
	require.NoError(t, os.WriteFile(parent1Path, []byte(parent1Content), 0o644))

	// Create second parent
	parent2Content := `
profile:
  name: compliance-baseline
  version: 1.0.0
vars:
  key: parent2_value
controls:
  items:
    - id: ctrl-1
      name: From Parent2
      observations:
        - plugin: http
    - id: ctrl-2
      name: Control 2
      observations:
        - plugin: dns
`
	parent2Path := filepath.Join(tmpDir, "compliance.yaml")
	require.NoError(t, os.WriteFile(parent2Path, []byte(parent2Content), 0o644))

	// Create child that extends both
	childContent := `
profile:
  name: production-combined
  version: 2.0.0
extends:
  - security.yaml
  - compliance.yaml
controls:
  items:
    - id: ctrl-3
      name: Control 3
      observations:
        - plugin: smtp
`
	childPath := filepath.Join(tmpDir, "production.yaml")
	require.NoError(t, os.WriteFile(childPath, []byte(childContent), 0o644))

	loader := NewProfileLoader()
	profile, err := loader.LoadProfile(childPath)

	require.NoError(t, err)
	assert.Equal(t, "production-combined", profile.Metadata.Name)

	// Vars: parent2 wins over parent1 (left-to-right)
	assert.Equal(t, "parent2_value", profile.Vars["key"])

	// ctrl-1 should be from parent2 (later parent wins)
	var ctrl1 *struct{ Name string }
	for _, ctrl := range profile.Controls.Items {
		if ctrl.ID == "ctrl-1" {
			ctrl1 = &struct{ Name string }{ctrl.Name}
			break
		}
	}
	require.NotNil(t, ctrl1)
	assert.Equal(t, "From Parent2", ctrl1.Name)
}

func TestLoadProfile_CircularDependencyDetected(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "profile-circular-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create profile A that extends B
	profileAContent := `
profile:
  name: profile-a
  version: 1.0.0
extends:
  - profile-b.yaml
controls:
  items:
    - id: ctrl-a
      name: Control A
      observations:
        - plugin: file
`
	profileAPath := filepath.Join(tmpDir, "profile-a.yaml")
	require.NoError(t, os.WriteFile(profileAPath, []byte(profileAContent), 0o644))

	// Create profile B that extends A (circular!)
	profileBContent := `
profile:
  name: profile-b
  version: 1.0.0
extends:
  - profile-a.yaml
controls:
  items:
    - id: ctrl-b
      name: Control B
      observations:
        - plugin: http
`
	profileBPath := filepath.Join(tmpDir, "profile-b.yaml")
	require.NoError(t, os.WriteFile(profileBPath, []byte(profileBContent), 0o644))

	loader := NewProfileLoader()
	_, err = loader.LoadProfile(profileAPath)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular inheritance detected")
}

func TestLoadProfile_MissingParentError(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "profile-missing-parent-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	childContent := `
profile:
  name: child
  version: 1.0.0
extends:
  - nonexistent-parent.yaml
controls:
  items:
    - id: ctrl-1
      name: Control 1
      observations:
        - plugin: file
`
	childPath := filepath.Join(tmpDir, "child.yaml")
	require.NoError(t, os.WriteFile(childPath, []byte(childContent), 0o644))

	loader := NewProfileLoader()
	_, err = loader.LoadProfile(childPath)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "loading parent")
	assert.Contains(t, err.Error(), "nonexistent-parent.yaml")
}

func TestLoadProfile_RelativePathResolution(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "profile-relative-path-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create subdirectory for base profiles
	basesDir := filepath.Join(tmpDir, "bases")
	require.NoError(t, os.MkdirAll(basesDir, 0o755))

	// Create base profile in subdirectory
	baseContent := `
profile:
  name: base
  version: 1.0.0
controls:
  items:
    - id: base-ctrl
      name: Base Control
      observations:
        - plugin: file
`
	basePath := filepath.Join(basesDir, "base.yaml")
	require.NoError(t, os.WriteFile(basePath, []byte(baseContent), 0o644))

	// Create child profile with relative path to base
	childContent := `
profile:
  name: child
  version: 1.0.0
extends:
  - bases/base.yaml
controls:
  items:
    - id: child-ctrl
      name: Child Control
      observations:
        - plugin: http
`
	childPath := filepath.Join(tmpDir, "child.yaml")
	require.NoError(t, os.WriteFile(childPath, []byte(childContent), 0o644))

	loader := NewProfileLoader()
	profile, err := loader.LoadProfile(childPath)

	require.NoError(t, err)
	assert.Equal(t, "child", profile.Metadata.Name)
	assert.Len(t, profile.Controls.Items, 2)

	controlIDs := make(map[string]bool)
	for _, ctrl := range profile.Controls.Items {
		controlIDs[ctrl.ID] = true
	}
	assert.True(t, controlIDs["base-ctrl"])
	assert.True(t, controlIDs["child-ctrl"])
}

func TestLoadProfile_NoExtends_ReturnsProfileAsIs(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "profile-no-extends-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	content := `
profile:
  name: simple
  version: 1.0.0
controls:
  items:
    - id: ctrl-1
      name: Control 1
      observations:
        - plugin: file
`
	profilePath := filepath.Join(tmpDir, "simple.yaml")
	require.NoError(t, os.WriteFile(profilePath, []byte(content), 0o644))

	loader := NewProfileLoader()
	profile, err := loader.LoadProfile(profilePath)

	require.NoError(t, err)
	assert.Equal(t, "simple", profile.Metadata.Name)
	assert.Len(t, profile.Controls.Items, 1)
}
