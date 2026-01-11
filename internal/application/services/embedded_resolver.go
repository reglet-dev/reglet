package services

import (
	"context"

	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/services"
	"github.com/reglet-dev/reglet/internal/domain/values"
)

// EmbeddedPluginResolver checks for built-in plugins.
type EmbeddedPluginResolver struct {
	services.BaseResolver
	source ports.EmbeddedPluginSource
}

// NewEmbeddedPluginResolver creates an embedded plugin resolver.
func NewEmbeddedPluginResolver(source ports.EmbeddedPluginSource) *EmbeddedPluginResolver {
	return &EmbeddedPluginResolver{
		source: source,
	}
}

// Resolve checks if plugin is embedded, otherwise delegates to next.
func (r *EmbeddedPluginResolver) Resolve(ctx context.Context, ref values.PluginReference) (*entities.Plugin, error) {
	// Only handle embedded plugins (simple name)
	if !ref.IsEmbedded() {
		return r.ResolveNext(ctx, ref)
	}

	path := r.source.Get(ref.Name())
	if path == "" {
		return r.ResolveNext(ctx, ref)
	}

	// Embedded plugins have synthetic metadata
	metadata := values.NewPluginMetadata(
		ref.Name(),
		"embedded",
		"Built-in plugin",
		[]string{}, // Capabilities loaded from WASM at runtime
	)

	// Digest for embedded plugins is computed from binary
	// (handled separately in infrastructure)
	digest, _ := values.NewDigest("sha256", "")

	plugin := entities.NewPlugin(ref, digest, metadata)
	return plugin, nil
}
