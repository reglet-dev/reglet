// Package config provides backward-compatible access to profile types.
// Domain types have moved to internal/domain/entities.
// This package will be deprecated after migration is complete.
package config

import (
	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
)

// DEPRECATED: Use entities.Profile instead.
// This package is transitional and will be removed.
type (
	Profile          = entities.Profile
	ProfileMetadata  = entities.ProfileMetadata
	ControlsSection  = entities.ControlsSection
	ControlDefaults  = entities.ControlDefaults
	Control          = entities.Control
	Observation      = entities.Observation
)