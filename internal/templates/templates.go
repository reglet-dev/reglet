// Package templates provides embedded templates for plugin scaffolding.
package templates

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"
	"text/template"
)

//go:embed go/*.tmpl
var goTemplates embed.FS

// PluginData contains the data used to render plugin templates.
type PluginData struct {
	// PluginName is the kebab-case name (e.g., "my-check")
	PluginName string
	// PluginTitle is the title case name (e.g., "My Check")
	PluginTitle string
	// PluginStructName is the PascalCase struct name (e.g., "myCheckPlugin")
	PluginStructName string
	// ModulePath is the Go module path (e.g., "github.com/user/my-check")
	ModulePath string
	// SDKVersion is the SDK version to use (e.g., "v0.1.0" or "latest")
	SDKVersion string
	// Capabilities is the list of required capabilities
	Capabilities []Capability
}

// Capability represents a plugin capability declaration.
type Capability struct {
	Kind    string
	Pattern string
}

// GoTemplates returns the parsed Go plugin templates.
func GoTemplates() (*template.Template, error) {
	tmpl := template.New("")

	err := fs.WalkDir(goTemplates, "go", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".tmpl") {
			return nil
		}

		content, err := goTemplates.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading template %s: %w", path, err)
		}

		// Use filename without .tmpl as template name
		name := strings.TrimPrefix(path, "go/")
		name = strings.TrimSuffix(name, ".tmpl")

		_, err = tmpl.New(name).Parse(string(content))
		if err != nil {
			return fmt.Errorf("parsing template %s: %w", path, err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("loading templates: %w", err)
	}

	return tmpl, nil
}

// TemplateFiles returns the list of template file names for a language.
func TemplateFiles(lang string) ([]string, error) {
	switch lang {
	case "go":
		return []string{
			"plugin.go",
			"main.go",
			"go.mod",
			"Makefile",
			"plugin_test.go",
			"README.md",
		}, nil
	default:
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}
}
