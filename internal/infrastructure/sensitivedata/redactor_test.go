package sensitivedata_test

import (
	"strings"
	"testing"

	"github.com/reglet-dev/reglet/internal/infrastructure/sensitivedata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactor_AWSKeyDetection(t *testing.T) {
	// Setup redactor with defaults
	redactor, err := sensitivedata.New(sensitivedata.Config{
		DisableGitleaks: true, // Use internal regex fallback
	})
	require.NoError(t, err)

	// Test case: AWS Access Key
	input := "My AWS key is AKIAIOSFODNN7EXAMPLE."
	expected := "My AWS key is [REDACTED]."
	got := redactor.ScrubString(input)

	assert.Equal(t, expected, got)
}

func TestRedactor_GitHubTokenDetection(t *testing.T) {
	// Setup redactor with defaults
	redactor, err := sensitivedata.New(sensitivedata.Config{
		DisableGitleaks: true, // Use internal regex fallback
	})
	require.NoError(t, err)

	// Test case: GitHub Token
	// Note: ghp_ pattern needs 36 chars
	token := "ghp_1234567890abcdefghijklmnopqrstuvwxyz"
	input := "My token is " + token
	expected := "My token is [REDACTED]"
	got := redactor.ScrubString(input)

	assert.Equal(t, expected, got)
}

func TestRedactor_HashMode(t *testing.T) {
	// Setup redactor with hash mode
	redactor, err := sensitivedata.New(sensitivedata.Config{
		DisableGitleaks: true,
		HashMode:        true,
		Salt:            "test-salt-123",
		Patterns:        []string{"secret"},
	})
	require.NoError(t, err)

	// Test case: Hashing
	input := "This is a secret message."
	got := redactor.ScrubString(input)

	assert.NotContains(t, got, "secret")
	assert.Contains(t, got, "[hmac:")
	// Ensure format [hmac:...]
	assert.True(t, strings.HasPrefix(strings.Split(got, " ")[3], "[hmac:"))
}
