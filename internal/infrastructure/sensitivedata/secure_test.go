package sensitivedata

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecureString(t *testing.T) {
	secret := "my-secret-password"
	ss := NewSecureString(secret)

	// Verify content
	assert.Equal(t, secret, ss.String())

	// Verify Zero()
	ss.Zero()
	assert.NotEqual(t, secret, ss.String())
	assert.Equal(t, string([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}), ss.String())
}
