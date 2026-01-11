package services

import (
	"context"
	"io"
	"log/slog"

	"github.com/reglet-dev/reglet/internal/application/dto"
	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/services"
	"github.com/reglet-dev/reglet/internal/domain/values"
)

// mockResolver implements PluginResolutionStrategy for testing
type mockResolver struct {
	services.BaseResolver
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

func (m *mockResolver) SetNext(next services.PluginResolutionStrategy) {
	m.BaseResolver.SetNext(next)
}

// MockRepository implements ports.PluginRepository
type MockRepository struct {
	FindPlugin *entities.Plugin
	FindPath   string
	FindErr    error

	StorePath string
	StoreErr  error

	ListPlugins []*entities.Plugin
	ListErr     error
}

func (m *MockRepository) Find(ctx context.Context, ref values.PluginReference) (*entities.Plugin, string, error) {
	if m.FindErr != nil {
		return nil, "", m.FindErr
	}
	return m.FindPlugin, m.FindPath, nil
}

func (m *MockRepository) Store(ctx context.Context, plugin *entities.Plugin, wasm io.Reader) (string, error) {
	if m.StoreErr != nil {
		return "", m.StoreErr
	}
	return m.StorePath, nil
}

func (m *MockRepository) List(ctx context.Context) ([]*entities.Plugin, error) {
	return m.ListPlugins, m.ListErr
}

func (m *MockRepository) Prune(ctx context.Context, keep int) error {
	return nil
}

func (m *MockRepository) Delete(ctx context.Context, ref values.PluginReference) error {
	return nil
}

// MockRegistry implements ports.PluginRegistry
type MockRegistry struct {
	PullArtifact *dto.PluginArtifactDTO
	PullErr      error
	PushErr      error
}

func (m *MockRegistry) Pull(ctx context.Context, ref values.PluginReference) (*dto.PluginArtifactDTO, error) {
	if m.PullErr != nil {
		return nil, m.PullErr
	}
	return m.PullArtifact, nil
}

func (m *MockRegistry) Push(ctx context.Context, artifact *dto.PluginArtifactDTO) error {
	return m.PushErr
}

func (m *MockRegistry) Resolve(ctx context.Context, ref values.PluginReference) (values.Digest, error) {
	// Dummy digest for mock
	d, _ := values.NewDigest("sha256", "mockdigest")
	return d, nil
}

// MockVerifier implements ports.IntegrityVerifier
type MockVerifier struct {
	VerifyResult *ports.SignatureResult
	VerifyErr    error
	SignErr      error
}

func (m *MockVerifier) VerifySignature(ctx context.Context, ref values.PluginReference) (*ports.SignatureResult, error) {
	if m.VerifyErr != nil {
		return nil, m.VerifyErr
	}
	// Return default success result if nil
	if m.VerifyResult == nil {
		return &ports.SignatureResult{
			Signer: "canonical",
		}, nil
	}
	return m.VerifyResult, nil
}

func (m *MockVerifier) Sign(ctx context.Context, ref values.PluginReference) error {
	return m.SignErr
}

// MockEmbeddedSource implements ports.EmbeddedPluginSource
type MockEmbeddedSource struct {
	Path string
}

func (m *MockEmbeddedSource) Get(name string) string {
	return m.Path
}

func (m *MockEmbeddedSource) List() []string {
	return []string{}
}

func NewTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
