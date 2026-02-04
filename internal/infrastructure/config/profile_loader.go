// Package config provides infrastructure for loading profile configurations.
// This package handles YAML parsing, file I/O, variable substitution, and profile inheritance.
package config

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/services"
)

// ProfileLoader handles loading profiles from YAML files with inheritance support.
//
// Inheritance Resolution:
//   - Profiles can specify parent profiles via the `extends` field
//   - Parents are loaded recursively and merged left-to-right
//   - Circular dependencies are detected and rejected
//   - Relative paths are resolved from the extending profile's directory
//
// # Cycle Detection Note
//
// This loader detects cycles in PROFILE INHERITANCE (extends field).
// This is different from Profile.CheckForCycles() which detects cycles
// in CONTROL DEPENDENCIES (depends_on field within a single profile).
//
// This is different from Profile.CheckForCycles() which detects cycles
// in CONTROL DEPENDENCIES (depends_on field within a single profile).
type ProfileLoader struct {
	merger *services.ProfileMerger
	fs     fs.FS
}

// ProfileLoaderOption defines a functional option for configuring ProfileLoader.
type ProfileLoaderOption func(*ProfileLoader)

// WithFilesystem configures the loader to use the provided filesystem.
// This is primarily used for testing or when loading profiles from
// non-standard locations (e.g., embedded files).
func WithFilesystem(fs fs.FS) ProfileLoaderOption {
	return func(l *ProfileLoader) {
		l.fs = fs
	}
}

// NewProfileLoader creates a new profile loader with optional configuration.
func NewProfileLoader(opts ...ProfileLoaderOption) *ProfileLoader {
	l := &ProfileLoader{
		merger: services.NewProfileMerger(),
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

// LoadProfile loads a profile and resolves all inheritance.
// This is the main entry point for profile loading.
func (l *ProfileLoader) LoadProfile(path string) (*entities.Profile, error) {
	visited := make(map[string]bool)
	return l.loadProfileRecursive(path, visited)
}

// loadProfileRecursive loads a profile and its parents recursively.
// It uses a visited map to detect circular dependencies.
func (l *ProfileLoader) loadProfileRecursive(
	path string,
	visited map[string]bool,
) (*entities.Profile, error) {
	// Resolve to absolute path for consistent tracking
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving path %q: %w", path, err)
	}

	// Circular dependency detection
	if visited[absPath] {
		return nil, fmt.Errorf("circular inheritance detected: %s", absPath)
	}
	visited[absPath] = true

	// Load current profile (I/O - infrastructure concern)
	current, err := l.loadSingleProfile(absPath)
	if err != nil {
		return nil, err
	}

	// No extends = no inheritance to resolve
	if len(current.Extends) == 0 {
		return current, nil
	}

	// Load all parent profiles
	parents := make([]*entities.Profile, 0, len(current.Extends))
	for _, parentPath := range current.Extends {
		resolvedPath := l.resolveRelativePath(absPath, parentPath)
		parent, err := l.loadProfileRecursive(resolvedPath, visited)
		if err != nil {
			return nil, fmt.Errorf("loading parent %q: %w", parentPath, err)
		}
		parents = append(parents, parent)
	}

	// Delegate to domain service for merge (business logic)
	return l.merger.MergeAll(parents, current), nil
}

// loadSingleProfile loads a single profile from disk without resolving inheritance.
func (l *ProfileLoader) loadSingleProfile(path string) (*entities.Profile, error) {
	if l.fs != nil {
		relPath := path
		if filepath.IsAbs(path) {
			// On unix this strips leading /. On windows stripping volume might be trickier but fs.FS is unix-style paths.
			// simplified: just remove leading separator if present to satisfy fs.Open validity checks.
			if len(path) > 0 && path[0] == filepath.Separator {
				relPath = path[1:]
			}
		}

		file, err := l.fs.Open(relPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open profile from fs: %w", err)
		}
		defer func() { _ = file.Close() }()
		return l.LoadProfileFromReader(file)
	}

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
// Note: This does NOT resolve inheritance, only parses YAML.
func (l *ProfileLoader) LoadProfileFromReader(r io.Reader) (*entities.Profile, error) {
	// Read all content first for potential re-parsing
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile: %w", err)
	}

	// First, do a loose parse to check for common format errors
	var rawCheck map[string]interface{}
	if err := yaml.Unmarshal(content, &rawCheck); err != nil {
		return nil, fmt.Errorf("failed to decode profile YAML: %w", err)
	}

	// Check for missing 'profile:' section (common mistake)
	if _, hasProfile := rawCheck["profile"]; !hasProfile {
		// Check if they put name/version at top level (wrong format)
		if _, hasName := rawCheck["name"]; hasName {
			return nil, fmt.Errorf("invalid profile format: 'name' and 'version' must be under 'profile:' section, not at top level. " +
				"Example:\n  profile:\n    name: my-profile\n    version: 1.0.0")
		}
		return nil, fmt.Errorf("invalid profile format: missing 'profile:' section. " +
			"Profile YAML must have 'profile:' with 'name:' and 'version:' fields")
	}

	// Now do the strict decode into the typed struct
	var cfg Profile
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("failed to decode profile YAML: %w", err)
	}

	// Note: We don't validate name/version here - that's done by ProfileCompiler
	// The loader only catches structural YAML issues (like missing profile: section)

	profile := cfg.ToEntity()
	return &profile, nil
}

// resolveRelativePath resolves a path relative to the current profile's directory.
// If extendsPath is absolute, it is returned as-is.
func (l *ProfileLoader) resolveRelativePath(currentPath, extendsPath string) string {
	if filepath.IsAbs(extendsPath) {
		return extendsPath
	}
	return filepath.Join(filepath.Dir(currentPath), extendsPath)
}
