package entities

import (
	"errors"
	"fmt"

	"github.com/reglet-dev/reglet/internal/domain/values"
)

// Sentinel errors for common error patterns.
// These allow both errors.Is() checks and errors.As() for detailed information.
var (
	// ErrPluginNotFound is returned when a plugin cannot be found in any source.
	ErrPluginNotFound = errors.New("plugin not found")

	// ErrIntegrityCheckFailed is returned when digest verification fails.
	ErrIntegrityCheckFailed = errors.New("integrity check failed")
)

// IntegrityError indicates digest mismatch.
// Provides detailed information about expected vs actual digest.
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

// Is implements error matching for errors.Is() checks.
// This allows: errors.Is(err, entities.ErrIntegrityCheckFailed)
func (e *IntegrityError) Is(target error) bool {
	return target == ErrIntegrityCheckFailed
}

// PluginNotFoundError indicates plugin doesn't exist in source.
// Provides detailed information about which plugin was not found.
type PluginNotFoundError struct {
	Reference values.PluginReference
}

func (e *PluginNotFoundError) Error() string {
	return fmt.Sprintf("plugin not found: %s", e.Reference.String())
}

// Is implements error matching for errors.Is() checks.
// This allows: errors.Is(err, entities.ErrPluginNotFound)
func (e *PluginNotFoundError) Is(target error) bool {
	return target == ErrPluginNotFound
}
