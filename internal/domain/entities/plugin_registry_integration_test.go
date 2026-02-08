package entities_test

import (
	"testing"

	hostEntities "github.com/reglet-dev/reglet-host-sdk/plugin/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPluginRegistry_AliasResolution tests comprehensive alias resolution scenarios.
func TestPluginRegistry_AliasResolution(t *testing.T) {
	t.Parallel()

	t.Run("resolves alias to versioned built-in", func(t *testing.T) {
		t.Parallel()
		registry := hostEntities.NewPluginRegistry()

		// Register an alias for a versioned built-in
		err := registry.Register(&hostEntities.PluginSpec{
			Name:    "file-v1",
			Source:  "file@1.0.0",
			Version: "1.0.0",
		})
		require.NoError(t, err)

		spec := registry.Resolve("file-v1")
		assert.Equal(t, "file-v1", spec.Name)
		assert.Equal(t, "file@1.0.0", spec.Source)
		assert.Equal(t, "file", spec.PluginName())
		assert.True(t, spec.IsBuiltIn())
	})

	t.Run("resolves alias to OCI reference", func(t *testing.T) {
		t.Parallel()
		registry := hostEntities.NewPluginRegistry()

		// Register an alias for an OCI reference
		err := registry.Register(&hostEntities.PluginSpec{
			Name:    "enterprise-scanner",
			Source:  "ghcr.io/acme-corp/scanner:2.1.0",
			Version: "2.1.0",
		})
		require.NoError(t, err)

		spec := registry.Resolve("enterprise-scanner")
		assert.Equal(t, "enterprise-scanner", spec.Name)
		assert.Equal(t, "ghcr.io/acme-corp/scanner:2.1.0", spec.Source)
		assert.Equal(t, "scanner", spec.PluginName())
		assert.False(t, spec.IsBuiltIn())
	})

	t.Run("resolves alias with digest pinning", func(t *testing.T) {
		t.Parallel()
		registry := hostEntities.NewPluginRegistry()

		err := registry.Register(&hostEntities.PluginSpec{
			Name:   "pinned-file",
			Source: "ghcr.io/reglet-dev/reglet-plugins/file@sha256:abc123def456",
			Digest: "sha256:abc123def456",
			Verify: true,
		})
		require.NoError(t, err)

		spec := registry.Resolve("pinned-file")
		assert.Equal(t, "sha256:abc123def456", spec.Digest)
		assert.True(t, spec.Verify)
	})

	t.Run("unregistered alias returns self-reference", func(t *testing.T) {
		t.Parallel()
		registry := hostEntities.NewPluginRegistry()

		spec := registry.Resolve("unregistered-plugin")
		assert.Equal(t, "unregistered-plugin", spec.Name)
		assert.Equal(t, "unregistered-plugin", spec.Source)
	})

	t.Run("multiple aliases for same source", func(t *testing.T) {
		t.Parallel()
		registry := hostEntities.NewPluginRegistry()

		// Same source with different aliases (e.g., for different environments)
		err := registry.Register(&hostEntities.PluginSpec{
			Name:   "file-prod",
			Source: "ghcr.io/reglet-dev/reglet-plugins/file:stable",
		})
		require.NoError(t, err)

		err = registry.Register(&hostEntities.PluginSpec{
			Name:   "file-dev",
			Source: "ghcr.io/reglet-dev/reglet-plugins/file:latest",
		})
		require.NoError(t, err)

		prodSpec := registry.Resolve("file-prod")
		devSpec := registry.Resolve("file-dev")

		assert.NotEqual(t, prodSpec.Source, devSpec.Source)
		assert.Equal(t, "file", prodSpec.PluginName())
		assert.Equal(t, "file", devSpec.PluginName())
	})
}

// TestPluginDeclaration_EdgeCases tests edge cases for plugin declaration parsing.
func TestPluginDeclaration_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("version with pre-release tag", func(t *testing.T) {
		t.Parallel()
		spec, err := hostEntities.ParsePluginDeclaration("file@1.0.0-alpha.1")
		require.NoError(t, err)
		assert.Equal(t, "file", spec.Name)
		assert.Equal(t, "1.0.0-alpha.1", spec.Version)
	})

	t.Run("OCI reference with org containing dots", func(t *testing.T) {
		t.Parallel()
		spec, err := hostEntities.ParsePluginDeclaration("registry.corp.com/security/scanner:1.0.0")
		require.NoError(t, err)
		assert.Equal(t, "scanner", spec.Name)
	})

	t.Run("digest with full sha256", func(t *testing.T) {
		t.Parallel()
		fullDigest := "ghcr.io/org/repo/plugin@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
		spec, err := hostEntities.ParsePluginDeclaration(fullDigest)
		require.NoError(t, err)
		assert.Equal(t, "plugin", spec.Name)
		assert.Equal(t, "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", spec.Digest)
	})
}

// TestPluginSpec_IntegrationWithRegistry tests plugin spec and registry integration.
func TestPluginSpec_IntegrationWithRegistry(t *testing.T) {
	t.Parallel()

	t.Run("profile plugins list populates registry", func(t *testing.T) {
		t.Parallel()

		// Simulate what happens when a profile is loaded
		pluginDeclarations := []string{
			"file",
			"http",
			"ghcr.io/reglet-dev/reglet-plugins/custom:1.0.0",
		}

		registry := hostEntities.NewPluginRegistry()
		for _, decl := range pluginDeclarations {
			spec, err := hostEntities.ParsePluginDeclaration(decl)
			require.NoError(t, err)
			err = registry.Register(spec)
			require.NoError(t, err)
		}

		// Verify all plugins are resolvable
		assert.True(t, registry.HasPlugin("file"))
		assert.True(t, registry.HasPlugin("http"))
		assert.True(t, registry.HasPlugin("custom"))

		// Verify specs have correct properties
		fileSpec := registry.Resolve("file")
		assert.True(t, fileSpec.IsBuiltIn())

		customSpec := registry.Resolve("custom")
		assert.False(t, customSpec.IsBuiltIn())
		assert.Equal(t, "1.0.0", customSpec.Version)
	})
}
