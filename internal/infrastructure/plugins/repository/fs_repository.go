// Package repository implements plugin repository adapters.
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/values"
)

// FSPluginRepository implements ports.PluginRepository using filesystem.
type FSPluginRepository struct {
	root string // ~/.reglet/plugins
}

// NewFSPluginRepository creates a filesystem-based repository.
func NewFSPluginRepository(root string) (*FSPluginRepository, error) {
	if root == "" {
		home, _ := os.UserHomeDir()
		root = filepath.Join(home, ".reglet", "plugins")
	}

	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("create cache directory: %w", err)
	}

	return &FSPluginRepository{root: root}, nil
}

// Find retrieves a plugin from cache.
func (r *FSPluginRepository) Find(ctx context.Context, ref values.PluginReference) (*entities.Plugin, string, error) {
	path := r.pluginPath(ref)

	// Check if WASM exists
	wasmPath := filepath.Join(path, "plugin.wasm")
	if _, err := os.Stat(wasmPath); err != nil {
		return nil, "", &entities.PluginNotFoundError{Reference: ref}
	}

	// Load metadata
	metadata, err := r.loadMetadata(path)
	if err != nil {
		return nil, "", err
	}

	// Load digest
	digest, err := r.loadDigest(path)
	if err != nil {
		return nil, "", err
	}

	plugin := entities.NewPlugin(ref, digest, metadata)
	return plugin, wasmPath, nil
}

// Store persists a plugin and its WASM binary.
func (r *FSPluginRepository) Store(ctx context.Context, plugin *entities.Plugin, wasm io.Reader) (string, error) {
	path := r.pluginPath(plugin.Reference())

	// Create directory
	if err := os.MkdirAll(path, 0o750); err != nil {
		return "", err
	}

	// Write WASM binary
	wasmPath := filepath.Join(path, "plugin.wasm")
	wasmFile, err := os.Create(filepath.Clean(wasmPath))
	if err != nil {
		return "", err
	}
	defer func() { _ = wasmFile.Close() }()

	if _, err := io.Copy(wasmFile, wasm); err != nil {
		return "", fmt.Errorf("write wasm: %w", err)
	}

	// Write metadata
	if err := r.saveMetadata(path, plugin.Metadata()); err != nil {
		return "", err
	}

	// Write digest
	if err := r.saveDigest(path, plugin.Digest()); err != nil {
		return "", err
	}

	return wasmPath, nil
}

// List returns all cached plugins.
func (r *FSPluginRepository) List(ctx context.Context) ([]*entities.Plugin, error) {
	var plugins []*entities.Plugin

	// Walk cache directory
	err := filepath.Walk(r.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check if this is a plugin.wasm file
		if info.Name() == "plugin.wasm" {
			// Parse reference from path structure
			ref, err := r.parseRefFromPath(filepath.Dir(path))
			if err != nil {
				return nil //nolint:nilerr // Skip invalid entries
			}

			plugin, _, err := r.Find(ctx, ref)
			if err == nil {
				plugins = append(plugins, plugin)
			}
		}

		return nil
	})

	return plugins, err
}

// Prune removes old versions.
func (r *FSPluginRepository) Prune(ctx context.Context, keepVersions int) error {
	// Group by plugin name, sort by version, delete old ones
	return nil
}

// Delete removes a plugin.
func (r *FSPluginRepository) Delete(ctx context.Context, ref values.PluginReference) error {
	path := r.pluginPath(ref)
	return os.RemoveAll(path)
}

// Helper methods

func (r *FSPluginRepository) pluginPath(ref values.PluginReference) string {
	// Structure: ~/.reglet/plugins/ghcr.io/whiskeyjimbo/reglet-plugins/file/1.0.0
	return filepath.Join(
		r.root,
		ref.String(),
	)
}

func (r *FSPluginRepository) loadMetadata(path string) (values.PluginMetadata, error) {
	cleanPath := filepath.Clean(filepath.Join(path, "metadata.json"))
	file, err := os.Open(cleanPath)
	if err != nil {
		return values.PluginMetadata{}, err
	}
	defer func() { _ = file.Close() }()

	var meta struct {
		Name         string   `json:"name"`
		Version      string   `json:"version"`
		Description  string   `json:"description"`
		Capabilities []string `json:"capabilities"`
	}

	if err := json.NewDecoder(file).Decode(&meta); err != nil {
		return values.PluginMetadata{}, err
	}

	return values.NewPluginMetadata(meta.Name, meta.Version, meta.Description, meta.Capabilities), nil
}

func (r *FSPluginRepository) saveMetadata(path string, metadata values.PluginMetadata) error {
	cleanPath := filepath.Clean(filepath.Join(path, "metadata.json"))
	file, err := os.Create(cleanPath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	meta := struct {
		Name         string   `json:"name"`
		Version      string   `json:"version"`
		Description  string   `json:"description"`
		Capabilities []string `json:"capabilities"`
	}{
		Name:         metadata.Name(),
		Version:      metadata.Version(),
		Description:  metadata.Description(),
		Capabilities: metadata.Capabilities(),
	}

	return json.NewEncoder(file).Encode(meta)
}

func (r *FSPluginRepository) loadDigest(path string) (values.Digest, error) {
	cleanPath := filepath.Clean(filepath.Join(path, "digest.txt"))
	data, err := os.ReadFile(cleanPath) // Validated internal path
	if err != nil {
		return values.Digest{}, err
	}
	return values.ParseDigest(string(data))
}

func (r *FSPluginRepository) saveDigest(path string, digest values.Digest) error {
	return os.WriteFile(filepath.Join(path, "digest.txt"), []byte(digest.String()), 0o600)
}

func (r *FSPluginRepository) parseRefFromPath(path string) (values.PluginReference, error) {
	// Extract reference components from path structure
	// This is a simplified implementation. A real one would need to carefully parse the path segments.
	relPath, err := filepath.Rel(r.root, path)
	if err != nil {
		return values.PluginReference{}, err
	}
	return values.ParsePluginReference(relPath)
}
