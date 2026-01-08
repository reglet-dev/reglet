package entities

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePluginDeclaration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		declaration string
		wantName    string
		wantSource  string
		wantVersion string
		wantDigest  string
		wantErr     bool
	}{
		{
			name:        "simple built-in",
			declaration: "file",
			wantName:    "file",
			wantSource:  "file",
		},
		{
			name:        "built-in with version",
			declaration: "file@1.2.0",
			wantName:    "file",
			wantSource:  "file",
			wantVersion: "1.2.0",
		},
		{
			name:        "OCI reference with tag",
			declaration: "ghcr.io/whiskeyjimbo/reglet-plugins/file:1.2.0",
			wantName:    "file",
			wantSource:  "ghcr.io/whiskeyjimbo/reglet-plugins/file:1.2.0",
			wantVersion: "1.2.0",
		},
		{
			name:        "OCI reference with digest",
			declaration: "ghcr.io/whiskeyjimbo/reglet-plugins/file@sha256:abc123",
			wantName:    "file",
			wantSource:  "ghcr.io/whiskeyjimbo/reglet-plugins/file@sha256:abc123",
			wantDigest:  "sha256:abc123",
		},
		{
			name:        "empty declaration",
			declaration: "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec, err := ParsePluginDeclaration(tt.declaration)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, spec.Name)
			assert.Equal(t, tt.wantSource, spec.Source)
			assert.Equal(t, tt.wantVersion, spec.Version)
			assert.Equal(t, tt.wantDigest, spec.Digest)
		})
	}
}

func TestParsePluginDeclarationWithAlias(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		alias      string
		source     interface{}
		wantName   string
		wantSource string
		wantDigest string
		wantVerify bool
		wantErr    bool
	}{
		{
			name:       "simple alias",
			alias:      "file-legacy",
			source:     "file@1.0.0",
			wantName:   "file-legacy",
			wantSource: "file",
		},
		{
			name:       "alias to OCI",
			alias:      "custom-file",
			source:     "ghcr.io/acme/file-plugin:2.0.0",
			wantName:   "custom-file",
			wantSource: "ghcr.io/acme/file-plugin:2.0.0",
		},
		{
			name:  "expanded format",
			alias: "enterprise",
			source: map[string]interface{}{
				"source": "registry.corp.com/scanner:1.0.0",
				"digest": "sha256:xyz789",
				"verify": true,
			},
			wantName:   "enterprise",
			wantSource: "registry.corp.com/scanner:1.0.0",
			wantDigest: "sha256:xyz789",
			wantVerify: true,
		},
		{
			name:    "empty alias",
			alias:   "",
			source:  "file",
			wantErr: true,
		},
		{
			name:    "expanded format missing source",
			alias:   "bad",
			source:  map[string]interface{}{"verify": true},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec, err := ParsePluginDeclarationWithAlias(tt.alias, tt.source)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, spec.Name)
			assert.Equal(t, tt.wantSource, spec.Source)
			assert.Equal(t, tt.wantDigest, spec.Digest)
			assert.Equal(t, tt.wantVerify, spec.Verify)
		})
	}
}

func TestPluginSpec_IsBuiltIn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		source    string
		isBuiltIn bool
	}{
		{"file", true},
		{"http", true},
		{"file@1.0.0", true}, // Versioned built-in is still built-in
		{"ghcr.io/foo/bar:1.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			t.Parallel()
			spec := &PluginSpec{Source: tt.source}
			assert.Equal(t, tt.isBuiltIn, spec.IsBuiltIn())
		})
	}
}

func TestPluginSpec_PluginName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		source   string
		wantName string
	}{
		{"file", "file"},
		{"file@1.0.0", "file"},
		{"ghcr.io/whiskeyjimbo/reglet-plugins/file:1.2.0", "file"},
		{"ghcr.io/whiskeyjimbo/reglet-plugins/file@sha256:abc", "file"},
		{"registry.corp.com/security/scanner:3.0.0", "scanner"},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			t.Parallel()
			spec := &PluginSpec{Source: tt.source}
			assert.Equal(t, tt.wantName, spec.PluginName())
		})
	}
}

func TestPluginRegistry(t *testing.T) {
	t.Parallel()

	registry := NewPluginRegistry()

	// Register plugins
	err := registry.Register(&PluginSpec{
		Name:   "file",
		Source: "file",
	})
	require.NoError(t, err)

	err = registry.Register(&PluginSpec{
		Name:   "file-legacy",
		Source: "file@1.0.0",
	})
	require.NoError(t, err)

	// Test resolution
	spec := registry.Resolve("file")
	assert.Equal(t, "file", spec.Name)
	assert.Equal(t, "file", spec.Source)

	spec = registry.Resolve("file-legacy")
	assert.Equal(t, "file-legacy", spec.Name)
	assert.Equal(t, "file@1.0.0", spec.Source)

	// Test unregistered alias returns default
	spec = registry.Resolve("unknown")
	assert.Equal(t, "unknown", spec.Name)
	assert.Equal(t, "unknown", spec.Source)

	// Test HasPlugin
	assert.True(t, registry.HasPlugin("file"))
	assert.True(t, registry.HasPlugin("file-legacy"))
	assert.False(t, registry.HasPlugin("unknown"))

	// Test AllSpecs
	allSpecs := registry.AllSpecs()
	assert.Len(t, allSpecs, 2)
}

func TestPluginRegistry_RegisterErrors(t *testing.T) {
	t.Parallel()

	registry := NewPluginRegistry()

	err := registry.Register(&PluginSpec{Name: "", Source: "file"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name cannot be empty")

	err = registry.Register(&PluginSpec{Name: "file", Source: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source cannot be empty")
}
