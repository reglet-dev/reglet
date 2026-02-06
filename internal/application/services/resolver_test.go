package services

import (
	"context"
	"errors"
	"testing"

	"github.com/reglet-dev/reglet/internal/application/dto"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/values"
)

func TestCachedPluginResolver(t *testing.T) {
	ref := values.NewPluginReference("reg", "org", "repo", "name", "1.0")
	plugin := entities.NewPlugin(ref, values.Digest{}, values.PluginMetadata{})

	t.Run("ReturnsCachedPlugin", func(t *testing.T) {
		repo := &MockRepository{FindPlugin: plugin}
		resolver := NewCachedPluginResolver(repo)

		got, err := resolver.Resolve(context.Background(), ref)
		if err != nil {
			t.Fatalf("Resolve failed: %v", err)
		}
		if got != plugin {
			t.Error("expected cached plugin")
		}
	})

	t.Run("DelegatesOnCacheMiss", func(t *testing.T) {
		repo := &MockRepository{FindErr: errors.New("not found")}
		resolver := NewCachedPluginResolver(repo)

		next := &mockResolver{err: errors.New("delegated")}
		resolver.SetNext(next)

		_, err := resolver.Resolve(context.Background(), ref)
		if err == nil || err.Error() != "delegated" {
			t.Errorf("expected delegation error, got %v", err)
		}
	})
}

func TestRegistryPluginResolver(t *testing.T) {
	logger := NewTestLogger()
	ref := values.NewPluginReference("reg", "org", "repo", "name", "1.0")
	plugin := entities.NewPlugin(ref, values.Digest{}, values.PluginMetadata{})
	artifact := dto.NewPluginArtifactDTO(plugin, nil)

	t.Run("PullAndCacheSuccess", func(t *testing.T) {
		registry := &MockRegistry{PullArtifact: artifact}
		repo := &MockRepository{}
		resolver := NewRegistryPluginResolver(registry, repo, logger)

		got, err := resolver.Resolve(context.Background(), ref)
		if err != nil {
			t.Fatalf("Resolve failed: %v", err)
		}
		if got != plugin {
			t.Error("expected pulled plugin")
		}
	})

	t.Run("PullFailure", func(t *testing.T) {
		registry := &MockRegistry{PullErr: errors.New("network error")}
		repo := &MockRepository{}
		resolver := NewRegistryPluginResolver(registry, repo, logger)

		_, err := resolver.Resolve(context.Background(), ref)
		if err == nil {
			t.Error("expected pull error")
		}
	})

	t.Run("CacheStorageFailure", func(t *testing.T) {
		registry := &MockRegistry{PullArtifact: artifact}
		repo := &MockRepository{StoreErr: errors.New("disk full")}
		resolver := NewRegistryPluginResolver(registry, repo, logger)

		_, err := resolver.Resolve(context.Background(), ref)
		if err == nil {
			t.Error("expected cache storage error")
		}
	})
}
