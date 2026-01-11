package ports

import (
	"context"

	"github.com/reglet-dev/reglet/internal/application/dto"
	"github.com/reglet-dev/reglet/internal/domain/values"
)

// PluginRegistry provides access to remote OCI registries.
type PluginRegistry interface {
	// Pull downloads a plugin artifact from the registry.
	Pull(ctx context.Context, ref values.PluginReference) (*dto.PluginArtifactDTO, error)

	// Push uploads a plugin artifact to the registry.
	Push(ctx context.Context, artifact *dto.PluginArtifactDTO) error

	// Resolve resolves a reference to its content digest.
	Resolve(ctx context.Context, ref values.PluginReference) (values.Digest, error)
}
