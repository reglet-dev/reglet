package hostfuncs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapabilityChecker_Check_Network(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		grants    []Capability
		kind      string
		pattern   string
		shouldErr bool
	}{
		{
			name: "exact port match",
			grants: []Capability{
				{Kind: "network", Pattern: "outbound:53"},
			},
			kind:      "network",
			pattern:   "outbound:53",
			shouldErr: false,
		},
		{
			name: "port in list",
			grants: []Capability{
				{Kind: "network", Pattern: "outbound:80,443"},
			},
			kind:      "network",
			pattern:   "outbound:80",
			shouldErr: false,
		},
		{
			name: "port in list (second port)",
			grants: []Capability{
				{Kind: "network", Pattern: "outbound:80,443"},
			},
			kind:      "network",
			pattern:   "outbound:443",
			shouldErr: false,
		},
		{
			name: "wildcard allows any port",
			grants: []Capability{
				{Kind: "network", Pattern: "outbound:*"},
			},
			kind:      "network",
			pattern:   "outbound:8080",
			shouldErr: false,
		},
		{
			name: "port not in list",
			grants: []Capability{
				{Kind: "network", Pattern: "outbound:80,443"},
			},
			kind:      "network",
			pattern:   "outbound:22",
			shouldErr: true,
		},
		{
			name: "wrong direction",
			grants: []Capability{
				{Kind: "network", Pattern: "outbound:80"},
			},
			kind:      "network",
			pattern:   "inbound:80",
			shouldErr: true,
		},
		{
			name: "no grants",
			grants: []Capability{},
			kind:      "network",
			pattern:   "outbound:80",
			shouldErr: true,
		},
		{
			name: "wrong kind grant",
			grants: []Capability{
				{Kind: "fs", Pattern: "read:/etc/**"},
			},
			kind:      "network",
			pattern:   "outbound:80",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			checker := NewCapabilityChecker(tt.grants)
			err := checker.Check(tt.kind, tt.pattern)

			if tt.shouldErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "capability denied")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCapabilityChecker_Check_Filesystem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		grants    []Capability
		kind      string
		pattern   string
		shouldErr bool
	}{
		{
			name: "recursive directory match",
			grants: []Capability{
				{Kind: "fs", Pattern: "read:/etc/**"},
			},
			kind:      "fs",
			pattern:   "read:/etc/ssh/sshd_config",
			shouldErr: false,
		},
		{
			name: "recursive directory match (direct child)",
			grants: []Capability{
				{Kind: "fs", Pattern: "read:/etc/**"},
			},
			kind:      "fs",
			pattern:   "read:/etc/hosts",
			shouldErr: false,
		},
		{
			name: "glob pattern match",
			grants: []Capability{
				{Kind: "fs", Pattern: "read:/var/log/*.log"},
			},
			kind:      "fs",
			pattern:   "read:/var/log/app.log",
			shouldErr: false,
		},
		{
			name: "exact path match",
			grants: []Capability{
				{Kind: "fs", Pattern: "read:/etc/hosts"},
			},
			kind:      "fs",
			pattern:   "read:/etc/hosts",
			shouldErr: false,
		},
		{
			name: "path not under granted directory",
			grants: []Capability{
				{Kind: "fs", Pattern: "read:/etc/**"},
			},
			kind:      "fs",
			pattern:   "read:/home/user/file",
			shouldErr: true,
		},
		{
			name: "wrong operation",
			grants: []Capability{
				{Kind: "fs", Pattern: "read:/etc/**"},
			},
			kind:      "fs",
			pattern:   "write:/etc/hosts",
			shouldErr: true,
		},
		{
			name: "glob pattern mismatch",
			grants: []Capability{
				{Kind: "fs", Pattern: "read:/var/log/*.log"},
			},
			kind:      "fs",
			pattern:   "read:/var/log/app.txt",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			checker := NewCapabilityChecker(tt.grants)
			err := checker.Check(tt.kind, tt.pattern)

			if tt.shouldErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "capability denied")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCapabilityChecker_Check_Environment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		grants    []Capability
		kind      string
		pattern   string
		shouldErr bool
	}{
		{
			name: "exact variable match",
			grants: []Capability{
				{Kind: "env", Pattern: "DB_PASSWORD"},
			},
			kind:      "env",
			pattern:   "DB_PASSWORD",
			shouldErr: false,
		},
		{
			name: "wildcard prefix match",
			grants: []Capability{
				{Kind: "env", Pattern: "AWS_*"},
			},
			kind:      "env",
			pattern:   "AWS_ACCESS_KEY_ID",
			shouldErr: false,
		},
		{
			name: "wildcard prefix match (another var)",
			grants: []Capability{
				{Kind: "env", Pattern: "AWS_*"},
			},
			kind:      "env",
			pattern:   "AWS_SECRET_ACCESS_KEY",
			shouldErr: false,
		},
		{
			name: "variable doesn't match prefix",
			grants: []Capability{
				{Kind: "env", Pattern: "AWS_*"},
			},
			kind:      "env",
			pattern:   "DB_PASSWORD",
			shouldErr: true,
		},
		{
			name: "exact match required",
			grants: []Capability{
				{Kind: "env", Pattern: "DB_PASSWORD"},
			},
			kind:      "env",
			pattern:   "DB_USERNAME",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			checker := NewCapabilityChecker(tt.grants)
			err := checker.Check(tt.kind, tt.pattern)

			if tt.shouldErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "capability denied")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCapabilityChecker_Check_Exec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		grants    []Capability
		kind      string
		pattern   string
		shouldErr bool
	}{
		{
			name: "exact binary path match",
			grants: []Capability{
				{Kind: "exec", Pattern: "/usr/bin/systemctl"},
			},
			kind:      "exec",
			pattern:   "/usr/bin/systemctl",
			shouldErr: false,
		},
		{
			name: "directory wildcard match",
			grants: []Capability{
				{Kind: "exec", Pattern: "/bin/*"},
			},
			kind:      "exec",
			pattern:   "/bin/ls",
			shouldErr: false,
		},
		{
			name: "directory wildcard match (another binary)",
			grants: []Capability{
				{Kind: "exec", Pattern: "/bin/*"},
			},
			kind:      "exec",
			pattern:   "/bin/cat",
			shouldErr: false,
		},
		{
			name: "binary not in granted directory",
			grants: []Capability{
				{Kind: "exec", Pattern: "/bin/*"},
			},
			kind:      "exec",
			pattern:   "/usr/bin/ls",
			shouldErr: true,
		},
		{
			name: "exact path mismatch",
			grants: []Capability{
				{Kind: "exec", Pattern: "/usr/bin/systemctl"},
			},
			kind:      "exec",
			pattern:   "/usr/bin/journalctl",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			checker := NewCapabilityChecker(tt.grants)
			err := checker.Check(tt.kind, tt.pattern)

			if tt.shouldErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "capability denied")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCapabilityChecker_Check_MultipleGrants(t *testing.T) {
	t.Parallel()

	grants := []Capability{
		{Kind: "network", Pattern: "outbound:53"},
		{Kind: "network", Pattern: "outbound:80,443"},
		{Kind: "fs", Pattern: "read:/etc/**"},
		{Kind: "env", Pattern: "AWS_*"},
	}

	checker := NewCapabilityChecker(grants)

	// Should allow DNS
	err := checker.Check("network", "outbound:53")
	require.NoError(t, err)

	// Should allow HTTP
	err = checker.Check("network", "outbound:80")
	require.NoError(t, err)

	// Should allow HTTPS
	err = checker.Check("network", "outbound:443")
	require.NoError(t, err)

	// Should deny SSH
	err = checker.Check("network", "outbound:22")
	require.Error(t, err)

	// Should allow reading /etc files
	err = checker.Check("fs", "read:/etc/hosts")
	require.NoError(t, err)

	// Should deny reading /home files
	err = checker.Check("fs", "read:/home/user/file")
	require.Error(t, err)

	// Should allow AWS env vars
	err = checker.Check("env", "AWS_ACCESS_KEY_ID")
	require.NoError(t, err)

	// Should deny other env vars
	err = checker.Check("env", "DB_PASSWORD")
	require.Error(t, err)
}

func TestCapabilityChecker_Check_UnknownKind(t *testing.T) {
	t.Parallel()

	grants := []Capability{
		{Kind: "network", Pattern: "outbound:80"},
	}

	checker := NewCapabilityChecker(grants)

	// Unknown capability kind should be denied
	err := checker.Check("unknown", "some:pattern")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "capability denied")
}

func TestMatchNetworkPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		requested string
		granted   string
		expected  bool
	}{
		{"exact match", "outbound:80", "outbound:80", true},
		{"port in list", "outbound:80", "outbound:80,443", true},
		{"port in list (spaces)", "outbound:80", "outbound:80, 443", true},
		{"wildcard", "outbound:8080", "outbound:*", true},
		{"port not in list", "outbound:22", "outbound:80,443", false},
		{"wrong direction", "inbound:80", "outbound:80", false},
		{"malformed request", "outbound", "outbound:80", false},
		{"malformed grant", "outbound:80", "outbound", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := matchNetworkPattern(tt.requested, tt.granted)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchFilesystemPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		requested string
		granted   string
		expected  bool
	}{
		{"exact match", "read:/etc/hosts", "read:/etc/hosts", true},
		{"recursive match", "read:/etc/ssh/sshd_config", "read:/etc/**", true},
		{"glob match", "read:/var/log/app.log", "read:/var/log/*.log", true},
		{"wrong operation", "write:/etc/hosts", "read:/etc/hosts", false},
		{"path not under grant", "read:/home/user", "read:/etc/**", false},
		{"glob mismatch", "read:/var/log/app.txt", "read:/var/log/*.log", false},
		{"malformed request", "read", "read:/etc/**", false},
		{"malformed grant", "read:/etc/hosts", "read", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := matchFilesystemPattern(tt.requested, tt.granted)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchEnvironmentPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		requested string
		granted   string
		expected  bool
	}{
		{"exact match", "DB_PASSWORD", "DB_PASSWORD", true},
		{"wildcard match", "AWS_ACCESS_KEY_ID", "AWS_*", true},
		{"wildcard match (another var)", "AWS_SECRET_ACCESS_KEY", "AWS_*", true},
		{"no match", "DB_PASSWORD", "AWS_*", false},
		{"partial match not allowed", "AWS", "AWS_*", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := matchEnvironmentPattern(tt.requested, tt.granted)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchExecPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		requested string
		granted   string
		expected  bool
	}{
		{"exact match", "/usr/bin/systemctl", "/usr/bin/systemctl", true},
		{"directory wildcard", "/bin/ls", "/bin/*", true},
		{"directory wildcard (another binary)", "/bin/cat", "/bin/*", true},
		{"wrong directory", "/usr/bin/ls", "/bin/*", false},
		{"exact mismatch", "/usr/bin/journalctl", "/usr/bin/systemctl", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := matchExecPattern(tt.requested, tt.granted)
			assert.Equal(t, tt.expected, result)
		})
	}
}
