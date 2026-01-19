package netutil_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/reglet-dev/reglet/internal/pkg/netutil"
)

func Test_StripCredentials(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no credentials",
			input: "https://example.com/path",
			want:  "https://example.com/path",
		},
		{
			name:  "with username only",
			input: "https://user@example.com/path",
			want:  "https://example.com/path",
		},
		{
			name:  "with username and password",
			input: "https://user:password@example.com/path",
			want:  "https://example.com/path",
		},
		{
			name:  "preserves query and fragment",
			input: "https://user:pass@example.com/path?foo=bar#section",
			want:  "https://example.com/path?foo=bar#section",
		},
		{
			name:  "simple path unchanged",
			input: "/just/a/path",
			want:  "/just/a/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, netutil.StripCredentials(tt.input))
		})
	}
}

func Test_HasCredentials(t *testing.T) {
	assert.True(t, netutil.HasCredentials("https://user:pass@example.com"))
	assert.True(t, netutil.HasCredentials("https://user@example.com"))
	assert.False(t, netutil.HasCredentials("https://example.com"))
	assert.False(t, netutil.HasCredentials("invalid"))
}

func Test_NormalizeURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "lowercase scheme",
			input: "HTTPS://Example.Com/Path",
			want:  "https://example.com/Path",
		},
		{
			name:  "removes default HTTPS port",
			input: "https://example.com:443/path",
			want:  "https://example.com/path",
		},
		{
			name:  "removes default HTTP port",
			input: "http://example.com:80/path",
			want:  "http://example.com/path",
		},
		{
			name:  "keeps non-default port",
			input: "https://example.com:8443/path",
			want:  "https://example.com:8443/path",
		},
		{
			name:  "strips credentials",
			input: "https://user:pass@example.com/path",
			want:  "https://example.com/path",
		},
		{
			name:  "sorts query parameters",
			input: "https://example.com/path?b=2&a=1",
			want:  "https://example.com/path?a=1&b=2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, netutil.NormalizeURL(tt.input))
		})
	}
}

func Test_ExtractHost(t *testing.T) {
	assert.Equal(t, "example.com", netutil.ExtractHost("https://example.com/path"))
	assert.Equal(t, "example.com:8443", netutil.ExtractHost("https://example.com:8443/path"))
	assert.Equal(t, "", netutil.ExtractHost("invalid"))
}

func Test_IsHTTPS(t *testing.T) {
	assert.True(t, netutil.IsHTTPS("https://example.com"))
	assert.True(t, netutil.IsHTTPS("HTTPS://example.com"))
	assert.False(t, netutil.IsHTTPS("http://example.com"))
	assert.False(t, netutil.IsHTTPS("oci://example.com"))
}

func Test_IsOCI(t *testing.T) {
	assert.True(t, netutil.IsOCI("oci://ghcr.io/org/profile"))
	assert.True(t, netutil.IsOCI("OCI://ghcr.io/org/profile"))
	assert.False(t, netutil.IsOCI("https://example.com"))
}
