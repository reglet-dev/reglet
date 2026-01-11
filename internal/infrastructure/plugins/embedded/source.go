package embedded

import (
	"embed"
	"path/filepath"
)

//go:embed wasm/*.wasm
var embeddedPlugins embed.FS

// EmbeddedSource implements ports.EmbeddedPluginSource.
type EmbeddedSource struct{}

// NewEmbeddedSource creates an embedded plugin source.
func NewEmbeddedSource() *EmbeddedSource {
	return &EmbeddedSource{}
}

// Get returns path to embedded plugin WASM.
func (s *EmbeddedSource) Get(name string) string {
	path := filepath.Join("wasm", name+".wasm")
	if _, err := embeddedPlugins.Open(path); err != nil {
		return ""
	}
	return path
}

// List returns all embedded plugin names.
func (s *EmbeddedSource) List() []string {
	entries, _ := embeddedPlugins.ReadDir("wasm")
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".wasm" {
			names = append(names, entry.Name()[:len(entry.Name())-5]) // Remove .wasm
		}
	}
	return names
}
