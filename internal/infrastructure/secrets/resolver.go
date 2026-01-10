// Package secrets deals with resolving sensitive values from external sources
// like environment variables and files.
package secrets

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/infrastructure/system"
)

// Resolver implements ports.SecretResolver.
// It resolves secrets from configured sources and automatically tracks them for redaction.
type Resolver struct {
	config   *system.SecretsConfig
	provider ports.SensitiveValueProvider // For auto-tracking
	cache    map[string]string
	mu       sync.RWMutex
}

// NewResolver creates a new secret resolver.
func NewResolver(
	config *system.SecretsConfig,
	provider ports.SensitiveValueProvider,
) *Resolver {
	return &Resolver{
		config:   config,
		provider: provider,
		cache:    make(map[string]string),
	}
}

// Resolve returns the secret value by name.
// It checks sources in order: Local -> Env -> Files.
// The resolved value is automatically tracked for redaction.
func (r *Resolver) Resolve(name string) (string, error) {
	r.mu.RLock()
	if value, ok := r.cache[name]; ok {
		r.mu.RUnlock()
		return value, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after write lock
	if value, ok := r.cache[name]; ok {
		return value, nil
	}

	value, err := r.resolveFromSources(name)
	if err != nil {
		return "", err
	}

	r.cache[name] = value
	r.provider.Track(value) // Auto-track for redaction
	return value, nil
}

func (r *Resolver) resolveFromSources(name string) (string, error) {
	if r.config == nil {
		return "", fmt.Errorf("secret %q: secrets config not present", name)
	}

	// 1. Check local secrets (dev only)
	if value, ok := r.config.Local[name]; ok {
		return value, nil
	}

	// 2. Check env var mapping
	if envVar, ok := r.config.Env[name]; ok {
		value := os.Getenv(envVar)
		if value == "" {
			return "", fmt.Errorf("secret %q: env var %q is not set", name, envVar)
		}
		return value, nil
	}

	// 3. Check file mapping (admin-controlled paths only)
	if filePath, ok := r.config.Files[name]; ok {
		// Security: Use os.OpenRoot to prevent path traversal
		dir := filepath.Dir(filePath)
		base := filepath.Base(filePath)

		root, err := os.OpenRoot(dir)
		if err != nil {
			return "", fmt.Errorf("secret %q: failed to open directory %q: %w", name, dir, err)
		}
		defer func() { _ = root.Close() }()

		f, err := root.Open(base)
		if err != nil {
			return "", fmt.Errorf("secret %q: failed to open file %q: %w", name, base, err)
		}
		defer func() { _ = f.Close() }()

		data, err := io.ReadAll(f)
		if err != nil {
			return "", fmt.Errorf("secret %q: reading file %q: %w", name, filePath, err)
		}
		return strings.TrimSpace(string(data)), nil
	}

	return "", fmt.Errorf("secret %q not found in local, env, or files", name)
}
