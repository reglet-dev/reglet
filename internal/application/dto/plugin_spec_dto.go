package dto

import "github.com/reglet-dev/reglet/internal/domain/values"

// PluginSpecDTO represents a plugin specification from configuration.
// Bridges Config context to Plugin Management context.
type PluginSpecDTO struct {
	Name   string // Plugin name or OCI reference
	Digest string // Expected digest (from lockfile, optional)
}

// ToPluginReference converts DTO to domain value object.
func (s *PluginSpecDTO) ToPluginReference() (values.PluginReference, error) {
	return values.ParsePluginReference(s.Name)
}

// ToDigest converts digest string to domain value object.
func (s *PluginSpecDTO) ToDigest() (values.Digest, error) {
	if s.Digest == "" {
		return values.Digest{}, nil
	}
	return values.ParseDigest(s.Digest)
}
