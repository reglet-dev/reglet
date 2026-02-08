package entities_test

import (
	"errors"
	"fmt"
	"testing"

	hostEntities "github.com/reglet-dev/reglet-host-sdk/plugin/entities"
	hostValues "github.com/reglet-dev/reglet-host-sdk/plugin/values"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginNotFoundError_ErrorMessage(t *testing.T) {
	ref := hostValues.NewPluginReference("ghcr.io", "example", "plugins", "my-plugin", "1.0.0")

	notFoundErr := &hostEntities.PluginNotFoundError{Reference: ref}

	assert.Contains(t, notFoundErr.Error(), "plugin not found")
	assert.Contains(t, notFoundErr.Error(), ref.String())
}

func TestPluginNotFoundError_ErrorIs(t *testing.T) {
	ref := hostValues.NewPluginReference("ghcr.io", "example", "plugins", "my-plugin", "1.0.0")

	notFoundErr := &hostEntities.PluginNotFoundError{Reference: ref}

	// Should support errors.Is() with sentinel error
	assert.True(t, errors.Is(notFoundErr, hostEntities.ErrPluginNotFound))

	// Should not match other sentinel errors
	assert.False(t, errors.Is(notFoundErr, hostEntities.ErrIntegrityCheckFailed))
}

func TestPluginNotFoundError_ErrorAs(t *testing.T) {
	ref := hostValues.NewPluginReference("ghcr.io", "example", "plugins", "my-plugin", "1.0.0")

	notFoundErr := &hostEntities.PluginNotFoundError{Reference: ref}

	// Wrap the error to simulate real-world usage
	wrappedErr := fmt.Errorf("failed to load plugin: %w", notFoundErr)

	// Should support errors.As() for type inspection
	var typedErr *hostEntities.PluginNotFoundError
	require.True(t, errors.As(wrappedErr, &typedErr))
	assert.Equal(t, ref.String(), typedErr.Reference.String())

	// Should also support errors.Is() when wrapped
	assert.True(t, errors.Is(wrappedErr, hostEntities.ErrPluginNotFound))
}

func TestIntegrityError_ErrorMessage(t *testing.T) {
	expected, err := hostValues.NewDigest("sha256", "abc123")
	require.NoError(t, err)

	actual, err := hostValues.NewDigest("sha256", "def456")
	require.NoError(t, err)

	integrityErr := &hostEntities.IntegrityError{
		Expected: expected,
		Actual:   actual,
	}

	assert.Contains(t, integrityErr.Error(), "integrity check failed")
	assert.Contains(t, integrityErr.Error(), "abc123")
	assert.Contains(t, integrityErr.Error(), "def456")
}

func TestIntegrityError_ErrorIs(t *testing.T) {
	expected, err := hostValues.NewDigest("sha256", "abc123")
	require.NoError(t, err)

	actual, err := hostValues.NewDigest("sha256", "def456")
	require.NoError(t, err)

	integrityErr := &hostEntities.IntegrityError{
		Expected: expected,
		Actual:   actual,
	}

	// Should support errors.Is() with sentinel error
	assert.True(t, errors.Is(integrityErr, hostEntities.ErrIntegrityCheckFailed))

	// Should not match other sentinel errors
	assert.False(t, errors.Is(integrityErr, hostEntities.ErrPluginNotFound))
}

func TestIntegrityError_ErrorAs(t *testing.T) {
	expected, err := hostValues.NewDigest("sha256", "abc123")
	require.NoError(t, err)

	actual, err := hostValues.NewDigest("sha256", "def456")
	require.NoError(t, err)

	integrityErr := &hostEntities.IntegrityError{
		Expected: expected,
		Actual:   actual,
	}

	// Wrap the error to simulate real-world usage
	wrappedErr := fmt.Errorf("plugin verification failed: %w", integrityErr)

	// Should support errors.As() for type inspection
	var typedErr *hostEntities.IntegrityError
	require.True(t, errors.As(wrappedErr, &typedErr))
	assert.Equal(t, expected.String(), typedErr.Expected.String())
	assert.Equal(t, actual.String(), typedErr.Actual.String())

	// Should also support errors.Is() when wrapped
	assert.True(t, errors.Is(wrappedErr, hostEntities.ErrIntegrityCheckFailed))
}

// TestErrorPatterns demonstrates the two supported error checking patterns.
func TestErrorPatterns(t *testing.T) {
	ref := hostValues.NewPluginReference("ghcr.io", "example", "plugins", "my-plugin", "1.0.0")

	notFoundErr := &hostEntities.PluginNotFoundError{Reference: ref}
	wrappedErr := fmt.Errorf("chain of responsibility exhausted: %w", notFoundErr)

	t.Run("Pattern 1: errors.Is() for quick type checks", func(t *testing.T) {
		// Simple check - is this any kind of "not found" error?
		if errors.Is(wrappedErr, hostEntities.ErrPluginNotFound) {
			// Handle plugin not found scenario
			assert.True(t, true, "Pattern 1 works")
		} else {
			t.Fatal("Pattern 1 should have matched")
		}
	})

	t.Run("Pattern 2: errors.As() for detailed information", func(t *testing.T) {
		// Need details - which plugin was not found?
		var notFound *hostEntities.PluginNotFoundError
		if errors.As(wrappedErr, &notFound) {
			// Can access detailed information
			assert.Equal(t, ref.String(), notFound.Reference.String())
		} else {
			t.Fatal("Pattern 2 should have matched")
		}
	})
}
