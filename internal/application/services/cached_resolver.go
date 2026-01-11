package services

import (
	"context"

	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/services"
	"github.com/reglet-dev/reglet/internal/domain/values"
)

// CachedPluginResolver checks local cache for plugins.
type CachedPluginResolver struct {
	services.BaseResolver
	repository ports.PluginRepository
}

// NewCachedPluginResolver creates a cached plugin resolver.
func NewCachedPluginResolver(repository ports.PluginRepository) *CachedPluginResolver {
	return &CachedPluginResolver{
		repository: repository,
	}
}

// Resolve checks cache, otherwise delegates to next.
func (r *CachedPluginResolver) Resolve(ctx context.Context, ref values.PluginReference) (*entities.Plugin, error) {
	plugin, _, err := r.repository.Find(ctx, ref)
	if err == nil {
		return plugin, nil // Found in cache
	}

	// Not in cache, try next resolver
	return r.ResolveNext(ctx, ref)
}
