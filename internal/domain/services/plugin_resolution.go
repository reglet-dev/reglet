package services

import (
	"context"

	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/values"
)

// PluginResolutionStrategy defines the interface for plugin resolution.
// Implements Chain of Responsibility pattern.
type PluginResolutionStrategy interface {
	// Resolve attempts to locate a plugin matching the reference.
	Resolve(ctx context.Context, ref values.PluginReference) (*entities.Plugin, error)

	// SetNext sets the next resolver in the chain.
	SetNext(next PluginResolutionStrategy)
}

// BaseResolver provides common chain-of-responsibility logic.
type BaseResolver struct {
	next PluginResolutionStrategy
}

// SetNext sets the next resolver in chain.
func (b *BaseResolver) SetNext(next PluginResolutionStrategy) {
	b.next = next
}

// ResolveNext delegates to next resolver in chain.
func (b *BaseResolver) ResolveNext(ctx context.Context, ref values.PluginReference) (*entities.Plugin, error) {
	if b.next == nil {
		return nil, &entities.PluginNotFoundError{Reference: ref}
	}
	return b.next.Resolve(ctx, ref)
}
