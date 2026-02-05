package plugins

import (
	"testing"

	"github.com/reglet-dev/reglet-sdk/domain/entities"
)

func TestNetworkExtractor_Extract(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]interface{}
		expected *entities.GrantSet
	}{
		{
			name: "HTTPS URL extracts host and port 443",
			config: map[string]interface{}{
				"url": "https://example.com/path",
			},
			expected: &entities.GrantSet{
				Network: &entities.NetworkCapability{
					Rules: []entities.NetworkRule{
						{Hosts: []string{"example.com"}, Ports: []string{"443"}},
					},
				},
			},
		},
		{
			name: "HTTP URL extracts host and port 80",
			config: map[string]interface{}{
				"url": "http://api.github.com/users",
			},
			expected: &entities.GrantSet{
				Network: &entities.NetworkCapability{
					Rules: []entities.NetworkRule{
						{Hosts: []string{"api.github.com"}, Ports: []string{"80"}},
					},
				},
			},
		},
		{
			name: "TCP with host and integer port",
			config: map[string]interface{}{
				"host": "example.com",
				"port": 22,
			},
			expected: &entities.GrantSet{
				Network: &entities.NetworkCapability{
					Rules: []entities.NetworkRule{
						{Hosts: []string{"example.com"}, Ports: []string{"22"}},
					},
				},
			},
		},
		{
			name: "TCP with IP and port",
			config: map[string]interface{}{
				"host": "192.168.1.1",
				"port": 3306,
			},
			expected: &entities.GrantSet{
				Network: &entities.NetworkCapability{
					Rules: []entities.NetworkRule{
						{Hosts: []string{"192.168.1.1"}, Ports: []string{"3306"}},
					},
				},
			},
		},
		{
			name: "TCP with port only (no host) uses wildcard",
			config: map[string]interface{}{
				"port": 22,
			},
			expected: &entities.GrantSet{
				Network: &entities.NetworkCapability{
					Rules: []entities.NetworkRule{
						{Hosts: []string{"*"}, Ports: []string{"22"}},
					},
				},
			},
		},
		{
			name: "TCP with string port uses wildcard host",
			config: map[string]interface{}{
				"port": "8080",
			},
			expected: &entities.GrantSet{
				Network: &entities.NetworkCapability{
					Rules: []entities.NetworkRule{
						{Hosts: []string{"*"}, Ports: []string{"8080"}},
					},
				},
			},
		},
		{
			name: "TCP with float64 port uses wildcard host",
			config: map[string]interface{}{
				"port": 3306.0,
			},
			expected: &entities.GrantSet{
				Network: &entities.NetworkCapability{
					Rules: []entities.NetworkRule{
						{Hosts: []string{"*"}, Ports: []string{"3306"}},
					},
				},
			},
		},
		{
			name: "TCP with uint64 port and host",
			config: map[string]interface{}{
				"host": "example.com",
				"port": uint64(443),
			},
			expected: &entities.GrantSet{
				Network: &entities.NetworkCapability{
					Rules: []entities.NetworkRule{
						{Hosts: []string{"example.com"}, Ports: []string{"443"}},
					},
				},
			},
		},
		{
			name: "DNS with custom nameserver",
			config: map[string]interface{}{
				"hostname":   "example.com",
				"nameserver": "8.8.8.8",
			},
			expected: &entities.GrantSet{
				Network: &entities.NetworkCapability{
					Rules: []entities.NetworkRule{
						{Hosts: []string{"8.8.8.8"}, Ports: []string{"53"}},
					},
				},
			},
		},
		{
			name: "DNS without nameserver returns nil",
			config: map[string]interface{}{
				"hostname":    "example.com",
				"record_type": "A",
			},
			expected: nil,
		},
		{
			name: "Invalid URL falls back to wildcard",
			config: map[string]interface{}{
				"url": "not-a-valid-url",
			},
			expected: &entities.GrantSet{
				Network: &entities.NetworkCapability{
					Rules: []entities.NetworkRule{
						{Hosts: []string{"*"}, Ports: []string{"443", "80"}},
					},
				},
			},
		},
		{
			name: "URL with unknown scheme uses both ports",
			config: map[string]interface{}{
				"url": "ftp://example.com/file",
			},
			expected: &entities.GrantSet{
				Network: &entities.NetworkCapability{
					Rules: []entities.NetworkRule{
						{Hosts: []string{"example.com"}, Ports: []string{"443", "80"}},
					},
				},
			},
		},
		{
			name:     "Empty config returns nil",
			config:   map[string]interface{}{},
			expected: nil,
		},
		{
			name: "Empty URL returns nil",
			config: map[string]interface{}{
				"url": "",
			},
			expected: nil,
		},
	}

	extractor := &NetworkExtractor{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractor.Extract(tt.config)

			if !grantSetsEqual(got, tt.expected) {
				t.Errorf("NetworkExtractor.Extract() = %+v, want %+v", got, tt.expected)
			}
		})
	}
}

func TestFileExtractor_Extract(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]interface{}
		expected *entities.GrantSet
	}{
		{
			name: "Valid path extracts read capability",
			config: map[string]interface{}{
				"path": "/etc/passwd",
			},
			expected: &entities.GrantSet{
				FS: &entities.FileSystemCapability{
					Rules: []entities.FileSystemRule{
						{Read: []string{"/etc/passwd"}},
					},
				},
			},
		},
		{
			name: "Empty path returns nil",
			config: map[string]interface{}{
				"path": "",
			},
			expected: nil,
		},
		{
			name:     "Missing path returns nil",
			config:   map[string]interface{}{},
			expected: nil,
		},
	}

	extractor := &FileExtractor{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractor.Extract(tt.config)

			if !grantSetsEqual(got, tt.expected) {
				t.Errorf("FileExtractor.Extract() = %+v, want %+v", got, tt.expected)
			}
		})
	}
}

func TestCommandExtractor_Extract(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]interface{}
		expected *entities.GrantSet
	}{
		{
			name: "Valid command extracts exec capability",
			config: map[string]interface{}{
				"command": "/bin/sh",
			},
			expected: &entities.GrantSet{
				Exec: &entities.ExecCapability{
					Commands: []string{"/bin/sh"},
				},
			},
		},
		{
			name: "Empty command returns nil",
			config: map[string]interface{}{
				"command": "",
			},
			expected: nil,
		},
		{
			name:     "Missing command returns nil",
			config:   map[string]interface{}{},
			expected: nil,
		},
	}

	extractor := &CommandExtractor{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractor.Extract(tt.config)

			if !grantSetsEqual(got, tt.expected) {
				t.Errorf("CommandExtractor.Extract() = %+v, want %+v", got, tt.expected)
			}
		})
	}
}

// Helper function to compare GrantSets
func grantSetsEqual(a, b *entities.GrantSet) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Compare Network
	if !networkCapsEqual(a.Network, b.Network) {
		return false
	}

	// Compare FS
	if !fsCapsEqual(a.FS, b.FS) {
		return false
	}

	// Compare Env
	if !envCapsEqual(a.Env, b.Env) {
		return false
	}

	// Compare Exec
	if !execCapsEqual(a.Exec, b.Exec) {
		return false
	}

	// Compare KV
	if !kvCapsEqual(a.KV, b.KV) {
		return false
	}

	return true
}

func networkCapsEqual(a, b *entities.NetworkCapability) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil || len(a.Rules) != len(b.Rules) {
		return false
	}
	for i := range a.Rules {
		if !networkRulesEqual(a.Rules[i], b.Rules[i]) {
			return false
		}
	}
	return true
}

func networkRulesEqual(a, b entities.NetworkRule) bool {
	if len(a.Hosts) != len(b.Hosts) || len(a.Ports) != len(b.Ports) {
		return false
	}
	for i := range a.Hosts {
		if a.Hosts[i] != b.Hosts[i] {
			return false
		}
	}
	for i := range a.Ports {
		if a.Ports[i] != b.Ports[i] {
			return false
		}
	}
	return true
}

func fsCapsEqual(a, b *entities.FileSystemCapability) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil || len(a.Rules) != len(b.Rules) {
		return false
	}
	for i := range a.Rules {
		if !fsRulesEqual(a.Rules[i], b.Rules[i]) {
			return false
		}
	}
	return true
}

func fsRulesEqual(a, b entities.FileSystemRule) bool {
	if len(a.Read) != len(b.Read) || len(a.Write) != len(b.Write) {
		return false
	}
	for i := range a.Read {
		if a.Read[i] != b.Read[i] {
			return false
		}
	}
	for i := range a.Write {
		if a.Write[i] != b.Write[i] {
			return false
		}
	}
	return true
}

func envCapsEqual(a, b *entities.EnvironmentCapability) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil || len(a.Variables) != len(b.Variables) {
		return false
	}
	for i := range a.Variables {
		if a.Variables[i] != b.Variables[i] {
			return false
		}
	}
	return true
}

func execCapsEqual(a, b *entities.ExecCapability) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil || len(a.Commands) != len(b.Commands) {
		return false
	}
	for i := range a.Commands {
		if a.Commands[i] != b.Commands[i] {
			return false
		}
	}
	return true
}

func kvCapsEqual(a, b *entities.KeyValueCapability) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil || len(a.Rules) != len(b.Rules) {
		return false
	}
	for i := range a.Rules {
		if !kvRulesEqual(a.Rules[i], b.Rules[i]) {
			return false
		}
	}
	return true
}

func kvRulesEqual(a, b entities.KeyValueRule) bool {
	if a.Operation != b.Operation || len(a.Keys) != len(b.Keys) {
		return false
	}
	for i := range a.Keys {
		if a.Keys[i] != b.Keys[i] {
			return false
		}
	}
	return true
}
