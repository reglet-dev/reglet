package entities

import (
	"fmt"
	"time"
)

// Lockfile is an aggregate root for reproducible plugin and profile resolution.
// It guarantees that plugin and profile versions are pinned for consistent builds.
//
// Invariants:
// - Each plugin entry must have a digest
// - Each profile entry must have a digest
// - Generated timestamp must be set
type Lockfile struct {
	Generated time.Time
	Plugins   map[string]PluginLock
	Profiles  map[string]ProfileLock
	Version   int
}

// PluginLock is a value object representing a pinned plugin version.
// Immutable after creation.
type PluginLock struct {
	Fetched   time.Time
	Modified  time.Time
	Requested string
	Resolved  string
	Source    string
	Digest    string
}

// ProfileLock is a value object representing a pinned remote profile version.
// Immutable after creation.
type ProfileLock struct {
	Fetched   time.Time
	Modified  time.Time
	Requested string // Original URL with version (e.g., "url#v1.2.0")
	Resolved  string // Actual version fetched
	Source    string // Normalized source URL
	Digest    string // Content hash (sha256:...)
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
	if (l.PluginCount() > 0 || l.ProfileCount() > 0) && l.Generated.IsZero() {
		return fmt.Errorf("generated timestamp is required")
	}
	for name, lock := range l.Plugins {
		if lock.Digest == "" {
			return fmt.Errorf("plugin %q: digest is required", name)
		}
	}
	for url, lock := range l.Profiles {
		if lock.Digest == "" {
			return fmt.Errorf("profile %q: digest is required", url)
		}
	}
	return nil
}

// PluginCount returns the number of locked plugins.
func (l *Lockfile) PluginCount() int {
	return len(l.Plugins)
}

// AddProfile adds a profile lock entry.
// Returns error if digest is empty (invariant enforcement).
func (l *Lockfile) AddProfile(url string, lock ProfileLock) error {
	if lock.Digest == "" {
		return fmt.Errorf("profile %q: digest is required", url)
	}
	if l.Profiles == nil {
		l.Profiles = make(map[string]ProfileLock)
	}
	l.Profiles[url] = lock
	return nil
}

// GetProfile retrieves a profile lock entry by URL.
// Returns nil if not found.
func (l *Lockfile) GetProfile(url string) *ProfileLock {
	if l.Profiles == nil {
		return nil
	}
	if lock, ok := l.Profiles[url]; ok {
		return &lock
	}
	return nil
}

// ProfileCount returns the number of locked profiles.
func (l *Lockfile) ProfileCount() int {
	return len(l.Profiles)
}
