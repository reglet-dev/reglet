package entities

import (
	"fmt"

	"github.com/reglet-dev/reglet/internal/domain/values"
)

// IntegrityError indicates digest mismatch.
type IntegrityError struct {
	Expected values.Digest
	Actual   values.Digest
}

func (e *IntegrityError) Error() string {
	return fmt.Sprintf(
		"integrity check failed: expected %s, got %s",
		e.Expected.String(),
		e.Actual.String(),
	)
}

// PluginNotFoundError indicates plugin doesn't exist in source.
type PluginNotFoundError struct {
	Reference values.PluginReference
}

func (e *PluginNotFoundError) Error() string {
	return fmt.Sprintf("plugin not found: %s", e.Reference.String())
}
