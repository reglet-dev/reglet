package capabilities

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Capability_IsBroad(t *testing.T) {
	tests := []struct {
		name       string
		capability Capability
		want       bool
	}{
		// Filesystem - broad patterns
		{
			name:       "fs wildcard all files",
			capability: Capability{Kind: "fs", Pattern: "**"},
			want:       true,
		},
		{
			name:       "fs root filesystem",
			capability: Capability{Kind: "fs", Pattern: "/**"},
			want:       true,
		},
		{
			name:       "fs read all",
			capability: Capability{Kind: "fs", Pattern: "read:**"},
			want:       true,
		},
		{
			name:       "fs write all",
			capability: Capability{Kind: "fs", Pattern: "write:**"},
			want:       true,
		},
		{
			name:       "fs sensitive /etc",
			capability: Capability{Kind: "fs", Pattern: "read:/etc/**"},
			want:       true,
		},
		{
			name:       "fs root home",
			capability: Capability{Kind: "fs", Pattern: "read:/root/**"},
			want:       true,
		},
		{
			name:       "fs all user homes",
			capability: Capability{Kind: "fs", Pattern: "read:/home/**"},
			want:       true,
		},

		// Filesystem - specific patterns (not broad)
		{
			name:       "fs specific file",
			capability: Capability{Kind: "fs", Pattern: "read:/etc/passwd"},
			want:       false,
		},
		{
			name:       "fs specific directory",
			capability: Capability{Kind: "fs", Pattern: "read:/var/log/app.log"},
			want:       false,
		},

		// Exec - broad patterns
		{
			name:       "exec wildcard",
			capability: Capability{Kind: "exec", Pattern: "**"},
			want:       true,
		},
		{
			name:       "exec asterisk",
			capability: Capability{Kind: "exec", Pattern: "*"},
			want:       true,
		},
		{
			name:       "exec bash",
			capability: Capability{Kind: "exec", Pattern: "bash"},
			want:       true,
		},
		{
			name:       "exec sh",
			capability: Capability{Kind: "exec", Pattern: "sh"},
			want:       true,
		},
		{
			name:       "exec python base",
			capability: Capability{Kind: "exec", Pattern: "python"},
			want:       true,
		},
		{
			name:       "exec python3",
			capability: Capability{Kind: "exec", Pattern: "python3"},
			want:       true,
		},
		{
			name:       "exec python3.11",
			capability: Capability{Kind: "exec", Pattern: "python3.11"},
			want:       true,
		},
		{
			name:       "exec node",
			capability: Capability{Kind: "exec", Pattern: "node"},
			want:       true,
		},
		{
			name:       "exec node18",
			capability: Capability{Kind: "exec", Pattern: "node18"},
			want:       true,
		},
		{
			name:       "exec ruby",
			capability: Capability{Kind: "exec", Pattern: "ruby"},
			want:       true,
		},
		{
			name:       "exec perl",
			capability: Capability{Kind: "exec", Pattern: "perl"},
			want:       true,
		},
		{
			name:       "exec php",
			capability: Capability{Kind: "exec", Pattern: "php"},
			want:       true,
		},
		{
			name:       "exec lua",
			capability: Capability{Kind: "exec", Pattern: "lua"},
			want:       true,
		},
		{
			name:       "exec awk",
			capability: Capability{Kind: "exec", Pattern: "awk"},
			want:       true,
		},
		{
			name:       "exec gawk",
			capability: Capability{Kind: "exec", Pattern: "gawk"},
			want:       true,
		},
		{
			name:       "exec irb",
			capability: Capability{Kind: "exec", Pattern: "irb"},
			want:       true,
		},

		// Exec - specific patterns (not broad)
		{
			name:       "exec specific binary",
			capability: Capability{Kind: "exec", Pattern: "/usr/bin/curl"},
			want:       false,
		},
		{
			name:       "exec specific command",
			capability: Capability{Kind: "exec", Pattern: "git"},
			want:       false,
		},
		{
			name:       "exec python with script",
			capability: Capability{Kind: "exec", Pattern: "python:/app/script.py"},
			want:       true, // Still broad - interpreter can execute arbitrary code
		},

		// Network - broad patterns
		{
			name:       "network wildcard",
			capability: Capability{Kind: "network", Pattern: "*"},
			want:       true,
		},
		{
			name:       "network outbound wildcard",
			capability: Capability{Kind: "network", Pattern: "outbound:*"},
			want:       true,
		},

		// Network - specific patterns (not broad)
		{
			name:       "network specific host",
			capability: Capability{Kind: "network", Pattern: "outbound:api.example.com"},
			want:       false,
		},
		{
			name:       "network specific host and port",
			capability: Capability{Kind: "network", Pattern: "outbound:api.example.com:443"},
			want:       false,
		},

		// Env - broad patterns
		{
			name:       "env wildcard",
			capability: Capability{Kind: "env", Pattern: "*"},
			want:       true,
		},
		{
			name:       "env AWS wildcard",
			capability: Capability{Kind: "env", Pattern: "AWS_*"},
			want:       true,
		},
		{
			name:       "env AZURE wildcard",
			capability: Capability{Kind: "env", Pattern: "AZURE_*"},
			want:       true,
		},
		{
			name:       "env GCP wildcard",
			capability: Capability{Kind: "env", Pattern: "GCP_*"},
			want:       true,
		},

		// Env - specific patterns (not broad)
		{
			name:       "env specific AWS key",
			capability: Capability{Kind: "env", Pattern: "AWS_ACCESS_KEY_ID"},
			want:       false,
		},
		{
			name:       "env specific var",
			capability: Capability{Kind: "env", Pattern: "HOME"},
			want:       false,
		},

		// Unknown kind - not broad
		{
			name:       "unknown kind",
			capability: Capability{Kind: "unknown", Pattern: "test"},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.capability.IsBroad()
			assert.Equal(t, tt.want, got, "IsBroad() = %v, want %v", got, tt.want)
		})
	}
}

func Test_Capability_RiskLevel(t *testing.T) {
	tests := []struct {
		name       string
		capability Capability
		want       RiskLevel
	}{
		// High risk - broad patterns
		{
			name:       "high risk - fs wildcard",
			capability: Capability{Kind: "fs", Pattern: "read:**"},
			want:       RiskLevelHigh,
		},
		{
			name:       "high risk - exec python",
			capability: Capability{Kind: "exec", Pattern: "python"},
			want:       RiskLevelHigh,
		},
		{
			name:       "high risk - network wildcard",
			capability: Capability{Kind: "network", Pattern: "*"},
			want:       RiskLevelHigh,
		},
		{
			name:       "high risk - env wildcard",
			capability: Capability{Kind: "env", Pattern: "*"},
			want:       RiskLevelHigh,
		},

		// Medium risk - network/exec even if specific
		{
			name:       "medium risk - network specific",
			capability: Capability{Kind: "network", Pattern: "outbound:api.example.com"},
			want:       RiskLevelMedium,
		},
		{
			name:       "medium risk - exec specific",
			capability: Capability{Kind: "exec", Pattern: "/usr/bin/curl"},
			want:       RiskLevelMedium,
		},
		{
			name:       "medium risk - fs read /etc",
			capability: Capability{Kind: "fs", Pattern: "read:/etc/passwd"},
			want:       RiskLevelMedium,
		},

		// Low risk - specific non-sensitive
		{
			name:       "low risk - fs specific file",
			capability: Capability{Kind: "fs", Pattern: "read:/var/log/app.log"},
			want:       RiskLevelLow,
		},
		{
			name:       "low risk - env specific",
			capability: Capability{Kind: "env", Pattern: "HOME"},
			want:       RiskLevelLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.capability.RiskLevel()
			assert.Equal(t, tt.want, got, "RiskLevel() = %v, want %v", got, tt.want)
		})
	}
}

func Test_Capability_RiskDescription(t *testing.T) {
	tests := []struct {
		name       string
		capability Capability
		contains   []string // strings that should be in the description
	}{
		{
			name:       "fs wildcard",
			capability: Capability{Kind: "fs", Pattern: "read:**"},
			contains:   []string{"ALL files", "system"},
		},
		{
			name:       "fs etc",
			capability: Capability{Kind: "fs", Pattern: "read:/etc/passwd"},
			contains:   []string{"sensitive", "system configuration"},
		},
		{
			name:       "fs home",
			capability: Capability{Kind: "fs", Pattern: "read:/home/user/data"},
			contains:   []string{"home directories", "private"},
		},
		{
			name:       "fs write",
			capability: Capability{Kind: "fs", Pattern: "write:/var/log/app.log"},
			contains:   []string{"modify files"},
		},
		{
			name:       "exec bash",
			capability: Capability{Kind: "exec", Pattern: "bash"},
			contains:   []string{"arbitrary", "shell commands"},
		},
		{
			name:       "exec python",
			capability: Capability{Kind: "exec", Pattern: "python3"},
			contains:   []string{"arbitrary code", "python", "interpreter"},
		},
		{
			name:       "exec specific",
			capability: Capability{Kind: "exec", Pattern: "/usr/bin/git"},
			contains:   []string{"execute specific command"},
		},
		{
			name:       "network wildcard",
			capability: Capability{Kind: "network", Pattern: "*"},
			contains:   []string{"any host", "internet"},
		},
		{
			name:       "network specific",
			capability: Capability{Kind: "network", Pattern: "outbound:api.example.com"},
			contains:   []string{"network requests", "api.example.com"},
		},
		{
			name:       "env wildcard",
			capability: Capability{Kind: "env", Pattern: "*"},
			contains:   []string{"ALL environment variables", "Secrets", "API keys"},
		},
		{
			name:       "env AWS wildcard",
			capability: Capability{Kind: "env", Pattern: "AWS_*"},
			contains:   []string{"ALL AWS", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"},
		},
		{
			name:       "env specific",
			capability: Capability{Kind: "env", Pattern: "HOME"},
			contains:   []string{"environment variable", "HOME"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.capability.RiskDescription()
			for _, substr := range tt.contains {
				assert.Contains(t, got, substr, "RiskDescription() should contain %q", substr)
			}
		})
	}
}

func Test_RiskLevel_String(t *testing.T) {
	tests := []struct {
		name string
		r    RiskLevel
		want string
	}{
		{"low", RiskLevelLow, "low"},
		{"medium", RiskLevelMedium, "medium"},
		{"high", RiskLevelHigh, "high"},
		{"unknown", RiskLevel(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.r.String()
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_isInterpreterVariant(t *testing.T) {
	tests := []struct {
		name            string
		pattern         string
		baseInterpreter string
		want            bool
	}{
		// Exact matches
		{"exact match", "python", "python", true},
		{"exact match node", "node", "node", true},

		// Versioned variants
		{"python3", "python3", "python", true},
		{"python3.11", "python3.11", "python", true},
		{"python2.7", "python2.7", "python", true},
		{"node18", "node18", "node", true},
		{"ruby3.2", "ruby3.2", "ruby", true},
		{"lua5.4", "lua5.4", "lua", true},

		// Subpath variants
		{"python with path", "python:/script.py", "python", true},
		{"python3 with path", "python3:/script.py", "python", true},
		{"node with wildcard", "node:*", "node", true},

		// Not matches
		{"different base", "ruby", "python", false},
		{"substring but not prefix", "my-python", "python", false},
		{"python in middle", "apython", "python", false},
		{"similar name", "pythonista", "python", false},
		{"with dash", "python-config", "python", false},
		{"full path", "/usr/bin/python", "python", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isInterpreterVariant(tt.pattern, tt.baseInterpreter)
			assert.Equal(t, tt.want, got, "isInterpreterVariant(%q, %q) = %v, want %v",
				tt.pattern, tt.baseInterpreter, got, tt.want)
		})
	}
}

func Test_matchesInterpreter(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    bool
	}{
		// Should match
		{"python", "python", true},
		{"python3", "python3", true},
		{"python3.11", "python3.11", true},
		{"node", "node", true},
		{"nodejs", "nodejs", true},
		{"node18", "node18", true},
		{"ruby", "ruby", true},
		{"perl", "perl", true},
		{"php", "php", true},
		{"lua", "lua", true},
		{"awk", "awk", true},
		{"gawk", "gawk", true},
		{"irb", "irb", true},

		// Should not match
		{"git", "git", false},
		{"curl", "curl", false},
		{"make", "make", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesInterpreter(tt.pattern)
			assert.Equal(t, tt.want, got, "matchesInterpreter(%q) = %v, want %v",
				tt.pattern, got, tt.want)
		})
	}
}

func Test_extractInterpreterName(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    string
	}{
		{"python", "python", "python"},
		{"python3", "python3", "python"},
		{"python3.11", "python3.11", "python"},
		{"node", "node", "node"},
		{"node18", "node18", "node"},
		{"ruby", "ruby", "ruby"},
		{"perl", "perl", "perl"},
		{"python with path", "python:/script.py", "python"},
		{"node with path", "node:/app.js", "node"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractInterpreterName(tt.pattern)
			assert.Equal(t, tt.want, got, "extractInterpreterName(%q) = %q, want %q",
				tt.pattern, got, tt.want)
		})
	}
}

func Test_matchesAny(t *testing.T) {
	list := []string{"foo", "bar", "baz"}

	tests := []struct {
		name    string
		pattern string
		want    bool
	}{
		{"matches first", "foo", true},
		{"matches middle", "bar", true},
		{"matches last", "baz", true},
		{"no match", "qux", false},
		{"empty pattern", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesAny(tt.pattern, list)
			assert.Equal(t, tt.want, got)
		})
	}
}
