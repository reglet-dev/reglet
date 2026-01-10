package capabilities

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPolicy_IsGranted_Network(t *testing.T) {
	policy := NewPolicy()

	tests := []struct {
		name      string
		grants    []Capability
		requested Capability
		expected  bool
	}{
		{
			name: "exact port match",
			grants: []Capability{
				{Kind: "network", Pattern: "outbound:53"},
			},
			requested: Capability{Kind: "network", Pattern: "outbound:53"},
			expected:  true,
		},
		{
			name: "port in list",
			grants: []Capability{
				{Kind: "network", Pattern: "outbound:80,443"},
			},
			requested: Capability{Kind: "network", Pattern: "outbound:80"},
			expected:  true,
		},
		{
			name: "wildcard allows any port",
			grants: []Capability{
				{Kind: "network", Pattern: "outbound:*"},
			},
			requested: Capability{Kind: "network", Pattern: "outbound:8080"},
			expected:  true,
		},
		{
			name: "port not in list",
			grants: []Capability{
				{Kind: "network", Pattern: "outbound:80,443"},
			},
			requested: Capability{Kind: "network", Pattern: "outbound:22"},
			expected:  false,
		},
		{
			name: "wrong direction",
			grants: []Capability{
				{Kind: "network", Pattern: "outbound:80"},
			},
			requested: Capability{Kind: "network", Pattern: "inbound:80"},
			expected:  false,
		},
		{
			name: "port in range",
			grants: []Capability{
				{Kind: "network", Pattern: "outbound:8000-9000"},
			},
			requested: Capability{Kind: "network", Pattern: "outbound:8500"},
			expected:  true,
		},
		{
			name: "port outside range",
			grants: []Capability{
				{Kind: "network", Pattern: "outbound:8000-9000"},
			},
			requested: Capability{Kind: "network", Pattern: "outbound:7999"},
			expected:  false,
		},
		{
			name: "url with hyphen",
			grants: []Capability{
				{Kind: "network", Pattern: "outbound:https://api-prod.example.com"},
			},
			requested: Capability{Kind: "network", Pattern: "outbound:https://api-prod.example.com"},
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, policy.IsGranted(tt.requested, tt.grants, ""))
		})
	}
}

func TestPolicy_IsGranted_Filesystem(t *testing.T) {
	policy := NewPolicy()

	tests := []struct {
		name      string
		grants    []Capability
		requested Capability
		expected  bool
	}{
		{
			name: "exact match",
			grants: []Capability{
				{Kind: "fs", Pattern: "read:/etc/hosts"},
			},
			requested: Capability{Kind: "fs", Pattern: "read:/etc/hosts"},
			expected:  true,
		},
		{
			name: "recursive match",
			grants: []Capability{
				{Kind: "fs", Pattern: "read:/etc/**"},
			},
			requested: Capability{Kind: "fs", Pattern: "read:/etc/ssh/sshd_config"},
			expected:  true,
		},
		{
			name: "glob match",
			grants: []Capability{
				{Kind: "fs", Pattern: "read:/var/log/*.log"},
			},
			requested: Capability{Kind: "fs", Pattern: "read:/var/log/app.log"},
			expected:  true,
		},
		{
			name: "wrong operation",
			grants: []Capability{
				{Kind: "fs", Pattern: "read:/etc/hosts"},
			},
			requested: Capability{Kind: "fs", Pattern: "write:/etc/hosts"},
			expected:  false,
		},
		{
			name: "parent directory traversal blocked",
			grants: []Capability{
				{Kind: "fs", Pattern: "read:/tmp/**"},
			},
			requested: Capability{Kind: "fs", Pattern: "read:/tmp/../etc/passwd"},
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, policy.IsGranted(tt.requested, tt.grants, ""))
		})
	}
}

func TestPolicy_IsGranted_Environment(t *testing.T) {
	policy := NewPolicy()

	tests := []struct {
		name      string
		grants    []Capability
		requested Capability
		expected  bool
	}{
		{
			name: "exact match",
			grants: []Capability{
				{Kind: "env", Pattern: "DB_PASSWORD"},
			},
			requested: Capability{Kind: "env", Pattern: "DB_PASSWORD"},
			expected:  true,
		},
		{
			name: "wildcard prefix",
			grants: []Capability{
				{Kind: "env", Pattern: "AWS_*"},
			},
			requested: Capability{Kind: "env", Pattern: "AWS_ACCESS_KEY_ID"},
			expected:  true,
		},
		{
			name: "no match",
			grants: []Capability{
				{Kind: "env", Pattern: "AWS_*"},
			},
			requested: Capability{Kind: "env", Pattern: "DB_PASSWORD"},
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, policy.IsGranted(tt.requested, tt.grants, ""))
		})
	}
}

func TestPolicy_IsGranted_Exec(t *testing.T) {
	policy := NewPolicy()

	tests := []struct {
		name      string
		grants    []Capability
		requested Capability
		expected  bool
	}{
		{
			name: "exact binary match",
			grants: []Capability{
				{Kind: "exec", Pattern: "/usr/bin/ls"},
			},
			requested: Capability{Kind: "exec", Pattern: "/usr/bin/ls"},
			expected:  true,
		},
		{
			name: "directory wildcard",
			grants: []Capability{
				{Kind: "exec", Pattern: "/bin/*"},
			},
			requested: Capability{Kind: "exec", Pattern: "/bin/ls"},
			expected:  true,
		},
		{
			name: "wrong directory",
			grants: []Capability{
				{Kind: "exec", Pattern: "/bin/*"},
			},
			requested: Capability{Kind: "exec", Pattern: "/usr/bin/ls"},
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, policy.IsGranted(tt.requested, tt.grants, ""))
		})
	}
}
