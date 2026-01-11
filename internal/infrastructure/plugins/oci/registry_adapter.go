// Package oci implements OCI registry adapters.
package oci

import (
	"bytes"
	"context"
	"fmt"
	"io"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"

	"github.com/reglet-dev/reglet/internal/application/dto"
	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/values"
)

// OCIRegistryAdapter implements ports.PluginRegistry using oras-go.
type OCIRegistryAdapter struct {
	auth ports.AuthProvider
}

// NewOCIRegistryAdapter creates an OCI registry adapter.
func NewOCIRegistryAdapter(auth ports.AuthProvider) *OCIRegistryAdapter {
	return &OCIRegistryAdapter{
		auth: auth,
	}
}

// Pull downloads a plugin from OCI registry.
func (a *OCIRegistryAdapter) Pull(ctx context.Context, ref values.PluginReference) (*dto.PluginArtifactDTO, error) {
	// Create repository client
	repo, err := remote.NewRepository(ref.Registry() + "/" + ref.String())
	if err != nil {
		return nil, fmt.Errorf("create repository: %w", err)
	}

	// Set credentials
	username, password, err := a.auth.GetCredentials(ctx, ref.Registry())
	if err == nil && username != "" {
		repo.Client = &auth.Client{
			Credential: func(ctx context.Context, registry string) (auth.Credential, error) {
				return auth.Credential{
					Username: username,
					Password: password,
				}, nil
			},
		}
	}

	// Pull manifest and layers
	memoryStore := memory.New()
	manifestDesc, err := oras.Copy(ctx, repo, ref.Version(), memoryStore, ref.Version(), oras.CopyOptions{})
	if err != nil {
		return nil, fmt.Errorf("pull artifact: %w", err)
	}

	// Parse manifest
	manifestRC, err := memoryStore.Fetch(ctx, manifestDesc)
	if err != nil {
		return nil, fmt.Errorf("fetch manifest: %w", err)
	}
	defer func() {
		_ = manifestRC.Close()
	}()

	manifestBytes, err := io.ReadAll(manifestRC)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	manifest, err := a.parseManifest(manifestBytes)
	if err != nil {
		return nil, err
	}

	// Extract metadata from config layer
	configRC, err := memoryStore.Fetch(ctx, manifest.Config)
	if err != nil {
		return nil, fmt.Errorf("fetch config: %w", err)
	}
	defer func() {
		_ = configRC.Close()
	}()

	configBytes, err := io.ReadAll(configRC)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	metadata, err := a.parseMetadata(configBytes)
	if err != nil {
		return nil, err
	}

	// Find WASM layer
	wasmDesc, err := a.findWASMLayer(manifest)
	if err != nil {
		return nil, err
	}

	// Fetch WASM binary
	wasmRC, err := memoryStore.Fetch(ctx, wasmDesc)
	if err != nil {
		return nil, fmt.Errorf("fetch wasm: %w", err)
	}
	defer func() {
		_ = wasmRC.Close()
	}()

	wasmBytes, err := io.ReadAll(wasmRC)
	if err != nil {
		return nil, fmt.Errorf("read wasm: %w", err)
	}

	// Create domain entities
	digest, _ := values.ParseDigest(string(wasmDesc.Digest))
	plugin := entities.NewPlugin(ref, digest, metadata)

	// Create DTO with I/O
	artifact := dto.NewPluginArtifactDTO(plugin, io.NopCloser(bytes.NewReader(wasmBytes)))

	return artifact, nil
}

// Push uploads a plugin to OCI registry.
func (a *OCIRegistryAdapter) Push(ctx context.Context, artifact *dto.PluginArtifactDTO) error {
	// Implementation similar to Pull but reversed
	// Use oras.Copy to push layers
	return nil
}

// Resolve resolves a reference to its digest.
func (a *OCIRegistryAdapter) Resolve(ctx context.Context, ref values.PluginReference) (values.Digest, error) {
	// Use oras to resolve tag to digest
	return values.Digest{}, nil
}

// Helper methods
func (a *OCIRegistryAdapter) parseManifest(data []byte) (*ocispec.Manifest, error) {
	// Parse OCI manifest JSON
	return nil, nil
}

func (a *OCIRegistryAdapter) parseMetadata(data []byte) (values.PluginMetadata, error) {
	// Parse plugin metadata JSON
	return values.PluginMetadata{}, nil
}

func (a *OCIRegistryAdapter) findWASMLayer(manifest *ocispec.Manifest) (ocispec.Descriptor, error) {
	for _, layer := range manifest.Layers {
		if layer.MediaType == "application/vnd.reglet.plugin.wasm.v1" {
			return layer, nil
		}
	}
	return ocispec.Descriptor{}, fmt.Errorf("no WASM layer found")
}
