package values_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/reglet-dev/reglet/internal/domain/values"
)

func Test_ParseProfileReference_ValidHTTPS(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantScheme  string
		wantHost    string
		wantPath    string
		wantVersion string
		wantDigest  bool
	}{
		{
			name:       "simple HTTPS URL",
			url:        "https://example.com/profile.yaml",
			wantScheme: "https",
			wantHost:   "example.com",
			wantPath:   "/profile.yaml",
		},
		{
			name:       "HTTPS URL with path",
			url:        "https://example.com/org/profiles/baseline.yaml",
			wantScheme: "https",
			wantHost:   "example.com",
			wantPath:   "/org/profiles/baseline.yaml",
		},
		{
			name:        "HTTPS URL with version fragment",
			url:         "https://example.com/profile.yaml#v1.2.0",
			wantScheme:  "https",
			wantHost:    "example.com",
			wantPath:    "/profile.yaml",
			wantVersion: "v1.2.0",
		},
		{
			name:       "HTTPS URL with digest",
			url:        "https://example.com/profile.yaml@sha256:abc123def456",
			wantScheme: "https",
			wantHost:   "example.com",
			wantPath:   "/profile.yaml",
			wantDigest: true,
		},
		{
			name:       "HTTPS URL with port",
			url:        "https://example.com:8443/profile.yaml",
			wantScheme: "https",
			wantHost:   "example.com:8443",
			wantPath:   "/profile.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := values.ParseProfileReference(tt.url)
			require.NoError(t, err)

			assert.Equal(t, tt.wantScheme, ref.Scheme())
			assert.Equal(t, tt.wantHost, ref.Host())
			assert.Equal(t, tt.wantPath, ref.Path())
			assert.Equal(t, tt.wantVersion, ref.Version())
			assert.True(t, ref.IsHTTPS())
			assert.False(t, ref.IsOCI())

			if tt.wantDigest {
				assert.True(t, ref.HasDigest())
				assert.NotNil(t, ref.Digest())
			} else {
				assert.False(t, ref.HasDigest())
			}
		})
	}
}

func Test_ParseProfileReference_ValidOCI(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantHost    string
		wantPath    string
		wantVersion string
	}{
		{
			name:     "OCI reference with tag in path",
			url:      "oci://ghcr.io/org/profiles/baseline:v1.0.0",
			wantHost: "ghcr.io",
			wantPath: "/org/profiles/baseline:v1.0.0",
		},
		{
			name:        "OCI reference with version fragment",
			url:         "oci://ghcr.io/org/profiles/baseline#v1.0.0",
			wantHost:    "ghcr.io",
			wantPath:    "/org/profiles/baseline",
			wantVersion: "v1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := values.ParseProfileReference(tt.url)
			require.NoError(t, err)

			assert.Equal(t, "oci", ref.Scheme())
			assert.Equal(t, tt.wantHost, ref.Host())
			assert.Equal(t, tt.wantPath, ref.Path())
			assert.Equal(t, tt.wantVersion, ref.Version())
			assert.True(t, ref.IsOCI())
			assert.False(t, ref.IsHTTPS())
		})
	}
}

func Test_ParseProfileReference_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr string
	}{
		{
			name:    "empty URL",
			url:     "",
			wantErr: "empty URL",
		},
		{
			name:    "HTTP URL not allowed",
			url:     "http://example.com/profile.yaml",
			wantErr: "HTTP URLs are not allowed",
		},
		{
			name:    "missing scheme",
			url:     "example.com/profile.yaml",
			wantErr: "missing URL scheme",
		},
		{
			name:    "unsupported scheme",
			url:     "ftp://example.com/profile.yaml",
			wantErr: "unsupported URL scheme",
		},
		{
			name:    "missing host",
			url:     "https:///profile.yaml",
			wantErr: "missing host",
		},
		{
			name:    "invalid digest format",
			url:     "https://example.com/profile.yaml@invalid:abc123",
			wantErr: "invalid digest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := values.ParseProfileReference(tt.url)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func Test_ProfileReference_CredentialStripping(t *testing.T) {
	// URL with credentials should have them stripped
	ref, err := values.ParseProfileReference("https://user:password@example.com/profile.yaml")
	require.NoError(t, err)

	// Credentials should not appear in any output
	assert.NotContains(t, ref.String(), "user")
	assert.NotContains(t, ref.String(), "password")
	assert.NotContains(t, ref.RawURL(), "user")
	assert.NotContains(t, ref.RawURL(), "password")
}

func Test_ProfileReference_CacheKey(t *testing.T) {
	// Same base URL should produce same cache key
	ref1, _ := values.ParseProfileReference("https://example.com/profile.yaml")
	ref2, _ := values.ParseProfileReference("https://example.com/profile.yaml#v1.0.0")
	ref3, _ := values.ParseProfileReference("https://example.com/profile.yaml#v2.0.0")

	// Version/digest should not affect cache key
	assert.Equal(t, ref1.CacheKey(), ref2.CacheKey())
	assert.Equal(t, ref2.CacheKey(), ref3.CacheKey())

	// Different URL should produce different cache key
	ref4, _ := values.ParseProfileReference("https://example.com/other.yaml")
	assert.NotEqual(t, ref1.CacheKey(), ref4.CacheKey())

	// Case insensitive for host
	ref5, _ := values.ParseProfileReference("https://EXAMPLE.COM/profile.yaml")
	assert.Equal(t, ref1.CacheKey(), ref5.CacheKey())
}

func Test_ProfileReference_String(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "simple URL",
			url:  "https://example.com/profile.yaml",
			want: "https://example.com/profile.yaml",
		},
		{
			name: "URL with version",
			url:  "https://example.com/profile.yaml#v1.2.0",
			want: "https://example.com/profile.yaml#v1.2.0",
		},
		{
			name: "URL with digest",
			url:  "https://example.com/profile.yaml@sha256:abc123",
			want: "https://example.com/profile.yaml@sha256:abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := values.ParseProfileReference(tt.url)
			require.NoError(t, err)
			assert.Equal(t, tt.want, ref.String())
		})
	}
}

func Test_ProfileReference_WithVersion(t *testing.T) {
	ref, _ := values.ParseProfileReference("https://example.com/profile.yaml")

	withVersion := ref.WithVersion("v2.0.0")
	assert.Equal(t, "v2.0.0", withVersion.Version())
	assert.False(t, withVersion.HasDigest())

	// Original should be unchanged
	assert.Empty(t, ref.Version())
}

func Test_ProfileReference_WithDigest(t *testing.T) {
	ref, _ := values.ParseProfileReference("https://example.com/profile.yaml#v1.0.0")

	digest, _ := values.NewDigest("sha256", "abc123def456")
	withDigest := ref.WithDigest(digest)

	assert.True(t, withDigest.HasDigest())
	assert.Empty(t, withDigest.Version())

	// Original should be unchanged
	assert.Equal(t, "v1.0.0", ref.Version())
}

func Test_ProfileReference_Equals(t *testing.T) {
	ref1, _ := values.ParseProfileReference("https://example.com/profile.yaml#v1.0.0")
	ref2, _ := values.ParseProfileReference("https://example.com/profile.yaml#v1.0.0")
	ref3, _ := values.ParseProfileReference("https://example.com/profile.yaml#v2.0.0")
	ref4, _ := values.ParseProfileReference("https://other.com/profile.yaml#v1.0.0")

	assert.True(t, ref1.Equals(ref2))
	assert.False(t, ref1.Equals(ref3))
	assert.False(t, ref1.Equals(ref4))
}

func Test_IsValidProfileURL(t *testing.T) {
	assert.True(t, values.IsValidProfileURL("https://example.com/profile.yaml"))
	assert.True(t, values.IsValidProfileURL("oci://ghcr.io/org/profile"))
	assert.False(t, values.IsValidProfileURL("http://example.com/profile.yaml"))
	assert.False(t, values.IsValidProfileURL("/local/path/profile.yaml"))
	assert.False(t, values.IsValidProfileURL("profile.yaml"))
}
