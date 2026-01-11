package services

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/services"
	"github.com/reglet-dev/reglet/internal/domain/values"
)

// RegistryPluginResolver pulls plugins from OCI registries.
type RegistryPluginResolver struct {
	services.BaseResolver
	registry   ports.PluginRegistry
	repository ports.PluginRepository
	logger     *slog.Logger
}

// NewRegistryPluginResolver creates a registry resolver.
func NewRegistryPluginResolver(
	registry ports.PluginRegistry,
	repository ports.PluginRepository,
	logger *slog.Logger,
) *RegistryPluginResolver {
	return &RegistryPluginResolver{
		registry:   registry,
		repository: repository,
		logger:     logger,
	}
}

// Resolve pulls from registry and caches.
func (r *RegistryPluginResolver) Resolve(ctx context.Context, ref values.PluginReference) (*entities.Plugin, error) {
	r.logger.Info("pulling plugin from registry", "ref", ref.String())

	// Pull artifact from registry
	artifact, err := r.registry.Pull(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("registry pull failed: %w", err)
	}
	defer func() {
		if cerr := artifact.Close(); cerr != nil {
			r.logger.Warn("failed to close artifact", "ref", ref.String(), "error", cerr)
		}
	}()

	// Store in cache
	_, err = r.repository.Store(ctx, artifact.Plugin, artifact.WASM)
	if err != nil {
		return nil, fmt.Errorf("cache storage failed: %w", err)
	}

	r.logger.Info("plugin cached", "ref", ref.String())

	return artifact.Plugin, nil
}
