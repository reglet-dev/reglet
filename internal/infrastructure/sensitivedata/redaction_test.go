package sensitivedata_test

import (
	"testing"

	"github.com/reglet-dev/reglet/internal/infrastructure/sensitivedata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactor_WithSensitiveProvider(t *testing.T) {
	// 1. Setup provider and track a secret
	provider := sensitivedata.NewProvider()
	secret := "super-secret-password-123"
	provider.Track(secret)

	// 2. Setup Redactor with provider
	cfg := sensitivedata.Config{
		DisableGitleaks: true, // Focus on our custom provider
	}
	redactor, err := sensitivedata.NewWithProvider(cfg, provider)
	require.NoError(t, err)

	// 3. Test redaction
	input := "The database password is super-secret-password-123."
	expected := "The database password is [REDACTED]."
	got := redactor.ScrubString(input)

	assert.Equal(t, expected, got)
}

func TestRedactor_DynamicTracking(t *testing.T) {
	// 1. Setup
	provider := sensitivedata.NewProvider()
	cfg := sensitivedata.Config{DisableGitleaks: true}
	redactor, err := sensitivedata.NewWithProvider(cfg, provider)
	require.NoError(t, err)

	// 2. Initial check - no secrets yet
	input := "My secret is dynamic-secret-999."
	assert.Equal(t, input, redactor.ScrubString(input), "Should not redact yet")

	// 3. Add secret dynamically
	provider.Track("dynamic-secret-999")

	// 4. Verify redaction now works
	expected := "My secret is [REDACTED]."
	assert.Equal(t, expected, redactor.ScrubString(input), "Should redact now")
}
