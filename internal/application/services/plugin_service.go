package services

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/reglet-dev/reglet/internal/application/dto"
	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/services"
)

// PluginService orchestrates plugin management use cases.
// Coordinates domain services and infrastructure adapters.
type PluginService struct {
	resolver          services.PluginResolutionStrategy
	repository        ports.PluginRepository
	registry          ports.PluginRegistry
	integrityVerifier ports.IntegrityVerifier
	integrityService  *services.IntegrityService
	logger            *slog.Logger
}

// PluginServiceOption configures a PluginService.
type PluginServiceOption func(*PluginService)

// NewPluginService creates a plugin service with the given options.
// Repository and registry are required dependencies.
func NewPluginService(
	repository ports.PluginRepository,
	registry ports.PluginRegistry,
	opts ...PluginServiceOption,
) *PluginService {
	s := &PluginService{
		repository:       repository,
		registry:         registry,
		logger:           slog.Default(),
		integrityService: services.NewIntegrityService(false),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// WithResolver sets the plugin resolution strategy.
func WithResolver(r services.PluginResolutionStrategy) PluginServiceOption {
	return func(s *PluginService) { s.resolver = r }
}

// WithIntegrityVerifier sets the integrity verifier.
func WithIntegrityVerifier(iv ports.IntegrityVerifier) PluginServiceOption {
	return func(s *PluginService) { s.integrityVerifier = iv }
}

// WithIntegrityService sets the integrity service.
func WithIntegrityService(is *services.IntegrityService) PluginServiceOption {
	return func(s *PluginService) { s.integrityService = is }
}

// WithLogger sets the logger.
func WithLogger(l *slog.Logger) PluginServiceOption {
	return func(s *PluginService) { s.logger = l }
}

// LoadPlugin is the main use case for loading a plugin.
// Returns the file path to the WASM binary.
func (s *PluginService) LoadPlugin(ctx context.Context, spec *dto.PluginSpecDTO) (string, error) {
	// Parse specification
	ref, err := spec.ToPluginReference()
	if err != nil {
		return "", fmt.Errorf("invalid plugin reference: %w", err)
	}

	expectedDigest, err := spec.ToDigest()
	if err != nil {
		return "", fmt.Errorf("invalid digest: %w", err)
	}

	// Resolve plugin using domain service (chain of responsibility)
	plugin, err := s.resolver.Resolve(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("plugin resolution failed: %w", err)
	}

	// Verify digest if provided (lockfile enforcement)
	if expectedDigest.Value() != "" {
		if err := s.integrityService.VerifyDigest(plugin, expectedDigest); err != nil {
			return "", fmt.Errorf("integrity verification failed: %w", err)
		}
	}

	// Verify signature if required by policy
	if s.integrityService.ShouldVerifySignature() {
		result, err := s.integrityVerifier.VerifySignature(ctx, ref)
		if err != nil {
			return "", fmt.Errorf("signature verification failed: %w", err)
		}
		s.logger.Info("plugin signature verified",
			"plugin", ref.String(),
			"signer", result.Signer,
			"signed_at", result.SignedAt)
	}

	// Get WASM path from repository
	_, wasmPath, err := s.repository.Find(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("failed to locate plugin binary: %w", err)
	}

	return wasmPath, nil
}

// PublishPlugin uploads a plugin to a registry.
func (s *PluginService) PublishPlugin(
	ctx context.Context,
	plugin *entities.Plugin,
	wasm io.Reader,
	shouldSign bool,
) error {
	// Create DTO for transport
	artifact := dto.NewPluginArtifactDTO(plugin, io.NopCloser(wasm))
	defer func() {
		if err := artifact.Close(); err != nil {
			s.logger.Warn("failed to close artifact", "ref", plugin.Reference().String(), "error", err)
		}
	}()

	// Push to registry
	if err := s.registry.Push(ctx, artifact); err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	// Sign if requested
	if shouldSign {
		if err := s.integrityVerifier.Sign(ctx, plugin.Reference()); err != nil {
			return fmt.Errorf("signing failed: %w", err)
		}
		s.logger.Info("plugin signed", "ref", plugin.Reference().String())
	}

	return nil
}

// ListCachedPlugins returns all plugins in local cache.
func (s *PluginService) ListCachedPlugins(ctx context.Context) ([]*entities.Plugin, error) {
	return s.repository.List(ctx)
}

// PruneCache removes old plugin versions.
func (s *PluginService) PruneCache(ctx context.Context, keepVersions int) error {
	return s.repository.Prune(ctx, keepVersions)
}
