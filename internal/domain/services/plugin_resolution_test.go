package services

import (
	"context"
	"errors"
	"testing"

	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/values"
)

// mockResolver implements PluginResolutionStrategy for testing
type mockResolver struct {
	BaseResolver
	foundPlugin *entities.Plugin
	err         error
	called      bool
}

func (m *mockResolver) Resolve(ctx context.Context, ref values.PluginReference) (*entities.Plugin, error) {
	m.called = true
	if m.err != nil {
		return nil, m.err
	}
	if m.foundPlugin != nil {
		return m.foundPlugin, nil
	}
	return m.ResolveNext(ctx, ref)
}

func TestBaseResolver_Chain(t *testing.T) {
	ref := values.NewPluginReference("reg", "org", "repo", "name", "1.0")

	t.Run("NextResolverCalled", func(t *testing.T) {
		r1 := &mockResolver{}
		r2 := &mockResolver{foundPlugin: &entities.Plugin{}}

		r1.SetNext(r2)

		got, err := r1.Resolve(context.Background(), ref)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil {
			t.Error("expected plugin, got nil")
		}
		if !r1.called {
			t.Error("r1 should be called")
		}
		if !r2.called {
			t.Error("r2 should be called via ResolveNext")
		}
	})

	t.Run("ChainEndsWithNotFoundError", func(t *testing.T) {
		r1 := &mockResolver{}
		// No next resolver

		_, err := r1.Resolve(context.Background(), ref)
		if err == nil {
			t.Error("expected error, got nil")
		}

		var notFoundErr *entities.PluginNotFoundError
		if !errors.As(err, &notFoundErr) {
			t.Errorf("expected PluginNotFoundError, got %T: %v", err, err)
		}
	})

	t.Run("ChainStopsOnFirstSuccess", func(t *testing.T) {
		r1 := &mockResolver{foundPlugin: &entities.Plugin{}}
		r2 := &mockResolver{foundPlugin: &entities.Plugin{}} // Should not be reached

		r1.SetNext(r2)

		_, err := r1.Resolve(context.Background(), ref)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !r1.called {
			t.Error("r1 should be called")
		}
		if r2.called {
			t.Error("r2 should NOT be called")
		}
	})

	t.Run("ChainPropagatesErrors", func(t *testing.T) {
		expectedErr := errors.New("resolution failure")
		r1 := &mockResolver{err: expectedErr}
		r2 := &mockResolver{}

		r1.SetNext(r2)

		_, err := r1.Resolve(context.Background(), ref)
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected %v, got %v", expectedErr, err)
		}
		if r2.called {
			t.Error("r2 should NOT be called on error")
		}
	})
}
