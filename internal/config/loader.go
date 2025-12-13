package config

import (
	"io"

	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
	infraconfig "github.com/whiskeyjimbo/reglet/internal/infrastructure/config"
)

// DEPRECATED: Use infraconfig.ProfileLoader instead.
// This function is transitional and will be removed.
func LoadProfile(path string) (*entities.Profile, error) {
	loader := infraconfig.NewProfileLoader()
	return loader.LoadProfile(path)
}

// DEPRECATED: Use infraconfig.ProfileLoader instead.
// This function is transitional and will be removed.
func LoadProfileFromReader(r io.Reader) (*entities.Profile, error) {
	loader := infraconfig.NewProfileLoader()
	return loader.LoadProfileFromReader(r)
}