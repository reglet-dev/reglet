package services_test

import (
	"context"
	"testing"

	"github.com/reglet-dev/reglet/internal/application/services"
	"github.com/reglet-dev/reglet/internal/domain/capability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfileTrustService_RequiresTrust(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"https URL requires trust", "https://example.com/profile.yaml", true},
		{"oci URL requires trust", "oci://ghcr.io/org/profile:v1", true},
		{"local path does not require trust", "./profile.yaml", false},
		{"absolute path does not require trust", "/path/to/profile.yaml", false},
		{"relative path does not require trust", "profile.yaml", false},
	}

	svc := services.NewProfileTrustService()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := svc.RequiresTrust(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProfileTrustService_IsTrusted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		patterns []string
		url      string
		expected bool
	}{
		{
			name:     "local path always trusted",
			patterns: nil,
			url:      "./profile.yaml",
			expected: true,
		},
		{
			name:     "remote URL not in patterns",
			patterns: nil,
			url:      "https://example.com/profile.yaml",
			expected: false,
		},
		{
			name:     "remote URL matches exact pattern",
			patterns: []string{"https://example.com/profile.yaml"},
			url:      "https://example.com/profile.yaml",
			expected: true,
		},
		{
			name:     "remote URL matches wildcard pattern",
			patterns: []string{"https://trusted.com/*"},
			url:      "https://trusted.com/profile.yaml",
			expected: true,
		},
		{
			name:     "remote URL does not match different pattern",
			patterns: []string{"https://trusted.com/*"},
			url:      "https://example.com/profile.yaml",
			expected: false,
		},
		{
			name:     "multiple patterns - matches second",
			patterns: []string{"https://first.com/*", "https://second.com/*"},
			url:      "https://second.com/profile.yaml",
			expected: true,
		},
		// Enhanced glob matching test cases
		{
			name:     "subdomain wildcard matches",
			patterns: []string{"https://*.example.com/*"},
			url:      "https://profiles.example.com/standard.yaml",
			expected: true,
		},
		{
			name:     "path-specific pattern matches",
			patterns: []string{"https://github.com/org/*/profiles/*"},
			url:      "https://github.com/org/repo/profiles/security.yaml",
			expected: true,
		},
		{
			name:     "path-specific pattern no match wrong path",
			patterns: []string{"https://example.com/profiles/*"},
			url:      "https://example.com/other/profile.yaml",
			expected: false,
		},
		{
			name:     "deep nested wildcard matches",
			patterns: []string{"https://cdn.example.com/*"},
			url:      "https://cdn.example.com/v1/policies/compliance/standard.yaml",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var opts []services.ProfileTrustServiceOption
			if len(tt.patterns) > 0 {
				opts = append(opts, services.WithTrustedSources(tt.patterns))
			}

			svc := services.NewProfileTrustService(opts...)
			result := svc.IsTrusted(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestProfileTrustService_PromptForTrust_WithTrustFlag(t *testing.T) {
	t.Parallel()

	svc := services.NewProfileTrustService()
	ctx := context.Background()

	// With trust flag, should always return true
	trusted, err := svc.PromptForTrust(ctx, "https://example.com/profile.yaml", nil, true)

	require.NoError(t, err)
	assert.True(t, trusted)
}

func TestProfileTrustService_PromptForTrust_AlreadyTrusted(t *testing.T) {
	t.Parallel()

	svc := services.NewProfileTrustService(
		services.WithTrustedSources([]string{"https://trusted.com/*"}),
	)
	ctx := context.Background()

	// URL matches trusted pattern, should return true without prompting
	trusted, err := svc.PromptForTrust(ctx, "https://trusted.com/profile.yaml", nil, false)

	require.NoError(t, err)
	assert.True(t, trusted)
}

func TestProfileTrustService_FormatNonInteractiveError(t *testing.T) {
	t.Parallel()

	svc := services.NewProfileTrustService()

	caps := map[string]capability.GrantSet{
		"file": {
			FS: &capability.FileSystemCapability{
				Rules: []capability.FileSystemRule{{Read: []string{"/etc/passwd"}}},
			},
		},
	}

	err := svc.FormatNonInteractiveError("https://example.com/profile.yaml", caps)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Remote profile requires trust approval")
	assert.Contains(t, err.Error(), "https://example.com/profile.yaml")
	assert.Contains(t, err.Error(), "--trust-source")
	assert.Contains(t, err.Error(), "/etc/passwd")
}
