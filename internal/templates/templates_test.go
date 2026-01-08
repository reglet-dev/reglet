package templates

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoTemplates_Load(t *testing.T) {
	t.Parallel()

	tmpl, err := GoTemplates()

	require.NoError(t, err)
	assert.NotNil(t, tmpl)

	// Verify all expected templates are loaded
	expectedTemplates := []string{
		"plugin.go",
		"main.go",
		"go.mod",
		"Makefile",
		"plugin_test.go",
		"README.md",
	}

	for _, name := range expectedTemplates {
		assert.NotNil(t, tmpl.Lookup(name), "template %s should be loaded", name)
	}
}

func TestTemplateFiles_Go(t *testing.T) {
	t.Parallel()

	files, err := TemplateFiles("go")

	require.NoError(t, err)
	assert.Len(t, files, 6)
	assert.Contains(t, files, "plugin.go")
	assert.Contains(t, files, "main.go")
	assert.Contains(t, files, "go.mod")
}

func TestTemplateFiles_Unsupported(t *testing.T) {
	t.Parallel()

	_, err := TemplateFiles("rust")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported language")
}

func TestGoTemplates_Render(t *testing.T) {
	t.Parallel()

	tmpl, err := GoTemplates()
	require.NoError(t, err)

	data := PluginData{
		PluginName:       "test-plugin",
		PluginTitle:      "Test Plugin",
		PluginStructName: "testPluginPlugin",
		ModulePath:       "github.com/test/test-plugin",
		SDKVersion:       "v0.1.0",
		Capabilities: []Capability{
			{Kind: "network", Pattern: "tcp"},
		},
	}

	// Test rendering plugin.go template
	buf := new(strings.Builder)
	err = tmpl.ExecuteTemplate(buf, "plugin.go", data)

	require.NoError(t, err)
	content := buf.String()
	assert.Contains(t, content, "testPluginPlugin")
	assert.Contains(t, content, `Name:        "test-plugin"`)
	assert.Contains(t, content, `{Kind: "network", Pattern: "tcp"}`)
}
