// Package config provides infrastructure for loading profile configurations.
// This package handles YAML parsing, file I/O, and variable substitution.
package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
)

// ProfileLoader handles loading profiles from YAML files.
type ProfileLoader struct{}

// NewProfileLoader creates a new profile loader.
func NewProfileLoader() *ProfileLoader {
	return &ProfileLoader{}
}

// LoadProfile loads and parses a profile from a YAML file.
// It applies control defaults and validates the profile structure.
func (l *ProfileLoader) LoadProfile(path string) (*entities.Profile, error) {
	// Security: Use os.OpenRoot to prevent path traversal attacks
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to open profile directory: %w", err)
	}
	defer func() {
		_ = root.Close() // Best-effort cleanup
	}()

	file, err := root.Open(base)
	if err != nil {
		return nil, fmt.Errorf("failed to open profile: %w", err)
	}
	defer func() {
		_ = file.Close() // Best-effort cleanup
	}()

	return l.LoadProfileFromReader(file)
}

// LoadProfileFromReader loads a profile from an io.Reader.
func (l *ProfileLoader) LoadProfileFromReader(r io.Reader) (*entities.Profile, error) {
	var profile entities.Profile

	decoder := yaml.NewDecoder(r)
	if err := decoder.Decode(&profile); err != nil {
		return nil, fmt.Errorf("failed to decode profile YAML: %w", err)
	}

	// Apply defaults to controls
	profile.ApplyDefaults()

	// Validate profile structure
	if err := profile.Validate(); err != nil {
		return nil, fmt.Errorf("profile validation failed: %w", err)
	}

	return &profile, nil
}
