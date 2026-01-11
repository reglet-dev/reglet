package filesystem

import (
	"time"

	"github.com/reglet-dev/reglet/internal/domain/entities"
)

// Lockfile represents the YAML structure of a lockfile.
type Lockfile struct {
	Generated time.Time             `yaml:"generated"`
	Plugins   map[string]PluginLock `yaml:"plugins"`
	Version   int                   `yaml:"lockfile_version"`
}

// PluginLock represents a pinned plugin version in YAML.
type PluginLock struct {
	Fetched   time.Time `yaml:"fetched,omitempty"`
	Modified  time.Time `yaml:"modified,omitempty"`
	Requested string    `yaml:"requested"`
	Resolved  string    `yaml:"resolved"`
	Source    string    `yaml:"source"`
	Digest    string    `yaml:"sha256"`
}

// ToEntity converts the lockfile to a domain entity.
func (l *Lockfile) ToEntity() *entities.Lockfile {
	entity := &entities.Lockfile{
		Generated: l.Generated,
		Version:   l.Version,
		Plugins:   make(map[string]entities.PluginLock, len(l.Plugins)),
	}

	for name, lock := range l.Plugins {
		entity.Plugins[name] = entities.PluginLock{
			Fetched:   lock.Fetched,
			Modified:  lock.Modified,
			Requested: lock.Requested,
			Resolved:  lock.Resolved,
			Source:    lock.Source,
			Digest:    lock.Digest,
		}
	}

	return entity
}

// FromEntity converts a domain lockfile to YAML representation.
func FromEntity(entity *entities.Lockfile) *Lockfile {
	if entity == nil {
		return nil
	}

	l := &Lockfile{
		Generated: entity.Generated,
		Version:   entity.Version,
		Plugins:   make(map[string]PluginLock, len(entity.Plugins)),
	}

	for name, lock := range entity.Plugins {
		l.Plugins[name] = PluginLock{
			Fetched:   lock.Fetched,
			Modified:  lock.Modified,
			Requested: lock.Requested,
			Resolved:  lock.Resolved,
			Source:    lock.Source,
			Digest:    lock.Digest,
		}
	}

	return l
}
