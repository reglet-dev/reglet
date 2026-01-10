package entities

import (
	"fmt"
	"time"
)

// Lockfile is an aggregate root for reproducible plugin resolution.
// It guarantees that plugin versions are pinned for consistent builds.
//
// Invariants:
// - Version must be 1 (current format version)
// - Each plugin entry must have a digest
// - Generated timestamp must be set
type Lockfile struct {
	Version   int                   `yaml:"lockfile_version"`
	Generated time.Time             `yaml:"generated"`
	Plugins   map[string]PluginLock `yaml:"plugins"`
}

// PluginLock is a value object representing a pinned plugin version.
// Immutable after creation.
type PluginLock struct {
	Requested string    `yaml:"requested"`          // Original constraint
	Resolved  string    `yaml:"resolved"`           // Exact version
	Source    string    `yaml:"source"`             // OCI ref or local path
	Digest    string    `yaml:"sha256"`             // SHA-256 hash
	Fetched   time.Time `yaml:"fetched,omitempty"`  // When fetched
	Modified  time.Time `yaml:"modified,omitempty"` // For local files
}

// NewLockfile creates a new lockfile with the current version.
func NewLockfile() *Lockfile {
	return &Lockfile{
		Version:   1,
		Generated: time.Now().UTC(),
		Plugins:   make(map[string]PluginLock),
	}
}

// AddPlugin adds a plugin lock entry.
// Returns error if digest is empty (invariant enforcement).
func (l *Lockfile) AddPlugin(name string, lock PluginLock) error {
	if lock.Digest == "" {
		return fmt.Errorf("plugin %q: digest is required", name)
	}
	if l.Plugins == nil {
		l.Plugins = make(map[string]PluginLock)
	}
	l.Plugins[name] = lock
	return nil
}

// GetPlugin retrieves a plugin lock entry by name.
// Returns nil if not found.
func (l *Lockfile) GetPlugin(name string) *PluginLock {
	if l.Plugins == nil {
		return nil
	}
	if lock, ok := l.Plugins[name]; ok {
		return &lock
	}
	return nil
}

// Validate checks lockfile invariants.
func (l *Lockfile) Validate() error {
	if l.Version != 1 {
		return fmt.Errorf("unsupported lockfile version: %d", l.Version)
	}
	if l.PluginCount() > 0 && l.Generated.IsZero() {
		return fmt.Errorf("generated timestamp is required")
	}
	for name, lock := range l.Plugins {
		if lock.Digest == "" {
			return fmt.Errorf("plugin %q: digest is required", name)
		}
	}
	return nil
}

// PluginCount returns the number of locked plugins.
func (l *Lockfile) PluginCount() int {
	return len(l.Plugins)
}
