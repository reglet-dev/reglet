package profiles_test

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/reglet-dev/reglet/internal/infrastructure/profiles"
)

func Test_HeaderAuthProvider_GetAuthHeader(t *testing.T) {
	ctx := context.Background()

	t.Run("returns empty for no matching rule", func(t *testing.T) {
		provider := profiles.NewHeaderAuthProvider([]profiles.AuthRule{
			{Pattern: "https://example.com/", AuthType: "bearer", Token: "token123"},
		})

		header, err := provider.GetAuthHeader(ctx, "https://other.com/profile.yaml")

		require.NoError(t, err)
		assert.Empty(t, header)
	})

	t.Run("returns bearer token", func(t *testing.T) {
		provider := profiles.NewHeaderAuthProvider([]profiles.AuthRule{
			{Pattern: "https://example.com/", AuthType: "bearer", Token: "my-secret-token"},
		})

		header, err := provider.GetAuthHeader(ctx, "https://example.com/profiles/test.yaml")

		require.NoError(t, err)
		assert.Equal(t, "Bearer my-secret-token", header)
	})

	t.Run("returns basic auth", func(t *testing.T) {
		provider := profiles.NewHeaderAuthProvider([]profiles.AuthRule{
			{Pattern: "https://example.com/", AuthType: "basic", Username: "user", Password: "pass"},
		})

		header, err := provider.GetAuthHeader(ctx, "https://example.com/profiles/test.yaml")

		require.NoError(t, err)
		expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
		assert.Equal(t, expected, header)
	})

	t.Run("returns raw header value", func(t *testing.T) {
		provider := profiles.NewHeaderAuthProvider([]profiles.AuthRule{
			{Pattern: "https://example.com/", AuthType: "header", HeaderValue: "Custom xyz123"},
		})

		header, err := provider.GetAuthHeader(ctx, "https://example.com/profiles/test.yaml")

		require.NoError(t, err)
		assert.Equal(t, "Custom xyz123", header)
	})

	t.Run("most specific pattern wins", func(t *testing.T) {
		provider := profiles.NewHeaderAuthProvider([]profiles.AuthRule{
			{Pattern: "https://example.com/", AuthType: "bearer", Token: "general"},
			{Pattern: "https://example.com/private/", AuthType: "bearer", Token: "specific"},
		})

		// Should match the more specific rule
		header, err := provider.GetAuthHeader(ctx, "https://example.com/private/secret.yaml")
		require.NoError(t, err)
		assert.Equal(t, "Bearer specific", header)

		// Should match the general rule
		header, err = provider.GetAuthHeader(ctx, "https://example.com/public/open.yaml")
		require.NoError(t, err)
		assert.Equal(t, "Bearer general", header)
	})
}

func Test_StaticBearerAuthProvider(t *testing.T) {
	ctx := context.Background()
	provider := profiles.NewStaticBearerAuthProvider("my-token")

	header, err := provider.GetAuthHeader(ctx, "https://any-url.com/anything")

	require.NoError(t, err)
	assert.Equal(t, "Bearer my-token", header)
}

func Test_StaticBasicAuthProvider(t *testing.T) {
	ctx := context.Background()
	provider := profiles.NewStaticBasicAuthProvider("admin", "secret")

	header, err := provider.GetAuthHeader(ctx, "https://any-url.com/anything")

	require.NoError(t, err)
	expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:secret"))
	assert.Equal(t, expected, header)
}

func Test_NoAuthProvider(t *testing.T) {
	ctx := context.Background()
	provider := &profiles.NoAuthProvider{}

	header, err := provider.GetAuthHeader(ctx, "https://example.com/profile.yaml")

	require.NoError(t, err)
	assert.Empty(t, header)
}

func Test_ChainAuthProvider(t *testing.T) {
	ctx := context.Background()

	t.Run("returns first non-empty header", func(t *testing.T) {
		provider := profiles.NewChainAuthProvider(
			&profiles.NoAuthProvider{},
			profiles.NewStaticBearerAuthProvider("fallback-token"),
		)

		header, err := provider.GetAuthHeader(ctx, "https://example.com/profile.yaml")

		require.NoError(t, err)
		assert.Equal(t, "Bearer fallback-token", header)
	})

	t.Run("returns empty if all providers return empty", func(t *testing.T) {
		provider := profiles.NewChainAuthProvider(
			&profiles.NoAuthProvider{},
			&profiles.NoAuthProvider{},
		)

		header, err := provider.GetAuthHeader(ctx, "https://example.com/profile.yaml")

		require.NoError(t, err)
		assert.Empty(t, header)
	})
}
