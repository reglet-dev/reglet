package profiles_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/reglet-dev/reglet/internal/infrastructure/profiles"
)

func Test_NewHTTPProfileFetcher(t *testing.T) {
	fetcher := profiles.NewHTTPProfileFetcher()

	assert.NotNil(t, fetcher)
	assert.Contains(t, fetcher.UserAgent, "reglet-profile-fetcher")
}

func Test_HTTPError(t *testing.T) {
	err := &profiles.HTTPError{
		StatusCode: 404,
		Status:     "Not Found",
		URL:        "https://user:pass@example.com/profile.yaml",
	}

	// Should include status code
	assert.Contains(t, err.Error(), "404")
	assert.Contains(t, err.Error(), "Not Found")

	// Should NOT include credentials in error message
	assert.NotContains(t, err.Error(), "user")
	assert.NotContains(t, err.Error(), "pass")

	// Should include the sanitized URL
	assert.Contains(t, err.Error(), "example.com")

	// Helper functions
	assert.True(t, profiles.IsHTTPError(err))
	assert.False(t, profiles.IsHTTPError(assert.AnError))
	assert.Equal(t, 404, profiles.GetHTTPStatusCode(err))
	assert.Equal(t, 0, profiles.GetHTTPStatusCode(assert.AnError))
}
