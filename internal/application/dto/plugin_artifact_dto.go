package dto

import (
	"io"

	"github.com/reglet-dev/reglet/internal/domain/entities"
)

// PluginArtifactDTO is a data transfer object for plugin artifacts.
// Contains I/O dependencies that don't belong in domain entities.
type PluginArtifactDTO struct {
	Plugin *entities.Plugin
	WASM   io.ReadCloser // Plugin binary stream
}

// NewPluginArtifactDTO creates a DTO from domain entity.
func NewPluginArtifactDTO(plugin *entities.Plugin, wasm io.ReadCloser) *PluginArtifactDTO {
	return &PluginArtifactDTO{
		Plugin: plugin,
		WASM:   wasm,
	}
}

// Close closes the WASM reader.
func (d *PluginArtifactDTO) Close() error {
	if d.WASM != nil {
		return d.WASM.Close()
	}
	return nil
}
