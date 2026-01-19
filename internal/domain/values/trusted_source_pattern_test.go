package values_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/reglet-dev/reglet/internal/domain/values"
)

func Test_ParseTrustedSourcePattern_Valid(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
	}{
		{
			name:    "HTTPS wildcard domain",
			pattern: "https://internal.company.com/*",
		},
		{
			name:    "HTTPS exact path",
			pattern: "https://example.com/profiles/baseline.yaml",
		},
		{
			name:    "OCI wildcard",
			pattern: "oci://ghcr.io/myorg/*",
		},
		{
			name:    "OCI with path pattern",
			pattern: "oci://ghcr.io/*/profiles/*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern, err := values.ParseTrustedSourcePattern(tt.pattern)
			require.NoError(t, err)
			assert.Equal(t, tt.pattern, pattern.String())
		})
	}
}

func Test_ParseTrustedSourcePattern_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		wantErr string
	}{
		{
			name:    "empty pattern",
			pattern: "",
			wantErr: "empty pattern",
		},
		{
			name:    "HTTP not allowed",
			pattern: "http://example.com/*",
			wantErr: "must start with https:// or oci://",
		},
		{
			name:    "missing scheme",
			pattern: "example.com/*",
			wantErr: "must start with https:// or oci://",
		},
		{
			name:    "invalid scheme",
			pattern: "ftp://example.com/*",
			wantErr: "must start with https:// or oci://",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := values.ParseTrustedSourcePattern(tt.pattern)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func Test_TrustedSourcePattern_Matches(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		url     string
		want    bool
	}{
		{
			name:    "wildcard matches any path",
			pattern: "https://internal.company.com/*",
			url:     "https://internal.company.com/profiles/baseline.yaml",
			want:    true,
		},
		{
			name:    "exact match",
			pattern: "https://example.com/profile.yaml",
			url:     "https://example.com/profile.yaml",
			want:    true,
		},
		{
			name:    "no match different host",
			pattern: "https://internal.company.com/*",
			url:     "https://external.company.com/profile.yaml",
			want:    false,
		},
		{
			name:    "no match different scheme",
			pattern: "https://example.com/*",
			url:     "oci://example.com/profile",
			want:    false,
		},
		{
			name:    "OCI wildcard matches",
			pattern: "oci://ghcr.io/myorg/*",
			url:     "oci://ghcr.io/myorg/profiles/baseline:v1.0",
			want:    true,
		},
		{
			name:    "version fragment ignored",
			pattern: "https://example.com/profile.yaml",
			url:     "https://example.com/profile.yaml#v1.2.0",
			want:    true,
		},
		{
			name:    "digest suffix ignored",
			pattern: "https://example.com/profile.yaml",
			url:     "https://example.com/profile.yaml@sha256:abc123",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern, err := values.ParseTrustedSourcePattern(tt.pattern)
			require.NoError(t, err)

			ref, err := values.ParseProfileReference(tt.url)
			require.NoError(t, err)

			assert.Equal(t, tt.want, pattern.Matches(ref))
		})
	}
}

func Test_TrustedSourcePattern_MatchesURL(t *testing.T) {
	pattern, err := values.ParseTrustedSourcePattern("https://example.com/*")
	require.NoError(t, err)

	assert.True(t, pattern.MatchesURL("https://example.com/profile.yaml"))
	assert.True(t, pattern.MatchesURL("https://example.com/profile.yaml#v1.0.0"))
	assert.False(t, pattern.MatchesURL("https://other.com/profile.yaml"))
}

func Test_TrustedSourcePatterns_Matches(t *testing.T) {
	patterns, err := values.ParseTrustedSourcePatterns([]string{
		"https://internal.company.com/*",
		"https://profiles.reglet.dev/*",
		"oci://ghcr.io/reglet-dev/*",
	})
	require.NoError(t, err)

	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "matches first pattern",
			url:  "https://internal.company.com/baseline.yaml",
			want: true,
		},
		{
			name: "matches second pattern",
			url:  "https://profiles.reglet.dev/security.yaml",
			want: true,
		},
		{
			name: "matches third pattern (OCI)",
			url:  "oci://ghcr.io/reglet-dev/profiles/baseline:v1.0",
			want: true,
		},
		{
			name: "no match",
			url:  "https://untrusted.com/profile.yaml",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := values.ParseProfileReference(tt.url)
			require.NoError(t, err)
			assert.Equal(t, tt.want, patterns.Matches(ref))
		})
	}
}

func Test_TrustedSourcePattern_Equals(t *testing.T) {
	p1, _ := values.ParseTrustedSourcePattern("https://example.com/*")
	p2, _ := values.ParseTrustedSourcePattern("https://example.com/*")
	p3, _ := values.ParseTrustedSourcePattern("https://other.com/*")

	assert.True(t, p1.Equals(p2))
	assert.False(t, p1.Equals(p3))
}
