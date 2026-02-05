package services

import (
	"testing"

	sdkEntities "github.com/reglet-dev/reglet-sdk/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/capabilities"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock extractors for testing (return GrantSet)
type testFileExtractor struct{}

func (e *testFileExtractor) Extract(config map[string]interface{}) *sdkEntities.GrantSet {
	if pathVal, ok := config["path"]; ok {
		if path, ok := pathVal.(string); ok && path != "" {
			return &sdkEntities.GrantSet{
				FS: &sdkEntities.FileSystemCapability{
					Rules: []sdkEntities.FileSystemRule{
						{Read: []string{path}},
					},
				},
			}
		}
	}
	return nil
}

type testCommandExtractor struct{}

func (e *testCommandExtractor) Extract(config map[string]interface{}) *sdkEntities.GrantSet {
	if cmdVal, ok := config["command"]; ok {
		if cmd, ok := cmdVal.(string); ok && cmd != "" {
			return &sdkEntities.GrantSet{
				Exec: &sdkEntities.ExecCapability{
					Commands: []string{cmd},
				},
			}
		}
	}
	return nil
}

type testNetworkExtractor struct{}

func (e *testNetworkExtractor) Extract(config map[string]interface{}) *sdkEntities.GrantSet {
	var hosts []string

	if urlVal, ok := config["url"]; ok {
		if url, ok := urlVal.(string); ok && url != "" {
			hosts = append(hosts, url)
		}
	}
	if hostVal, ok := config["host"]; ok {
		if host, ok := hostVal.(string); ok && host != "" {
			hosts = append(hosts, host)
		}
	}

	if len(hosts) == 0 {
		return nil
	}

	return &sdkEntities.GrantSet{
		Network: &sdkEntities.NetworkCapability{
			Rules: []sdkEntities.NetworkRule{
				{Hosts: hosts, Ports: []string{"*"}},
			},
		},
	}
}

func setupTestRegistry() *capabilities.Registry {
	r := capabilities.NewRegistry()
	r.Register("file", &testFileExtractor{})
	r.Register("command", &testCommandExtractor{})
	net := &testNetworkExtractor{}
	r.Register("http", net)
	r.Register("tcp", net)
	r.Register("dns", net)
	return r
}

func TestCapabilityAnalyzer_ExtractCapabilities_FilePlugin(t *testing.T) {
	analyzer := NewCapabilityAnalyzer(setupTestRegistry())

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "test", Version: "1.0.0"},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID: "file-check",
					ObservationDefinitions: []entities.ObservationDefinition{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/etc/passwd",
							},
						},
					},
				},
			},
		},
	}

	caps := analyzer.ExtractCapabilities(profile)

	assert.Contains(t, caps, "file")
	require.NotNil(t, caps["file"])
	require.NotNil(t, caps["file"].FS)
	require.Len(t, caps["file"].FS.Rules, 1)
	assert.Contains(t, caps["file"].FS.Rules[0].Read, "/etc/passwd")
}

func TestCapabilityAnalyzer_ExtractCapabilities_CommandPlugin(t *testing.T) {
	analyzer := NewCapabilityAnalyzer(setupTestRegistry())

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "test", Version: "1.0.0"},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID: "cmd-check",
					ObservationDefinitions: []entities.ObservationDefinition{
						{
							Plugin: "command",
							Config: map[string]interface{}{
								"command": "/usr/bin/systemctl",
							},
						},
					},
				},
			},
		},
	}

	caps := analyzer.ExtractCapabilities(profile)

	assert.Contains(t, caps, "command")
	require.NotNil(t, caps["command"])
	require.NotNil(t, caps["command"].Exec)
	assert.Contains(t, caps["command"].Exec.Commands, "/usr/bin/systemctl")
}

func TestCapabilityAnalyzer_ExtractCapabilities_NetworkPlugins(t *testing.T) {
	tests := []struct {
		name       string
		pluginName string
		config     map[string]interface{}
		expected   string // Expected host in Network.Rules[0].Hosts
	}{
		{
			name:       "HTTP with URL",
			pluginName: "http",
			config: map[string]interface{}{
				"url": "https://api.example.com",
			},
			expected: "https://api.example.com",
		},
		{
			name:       "TCP with host",
			pluginName: "tcp",
			config: map[string]interface{}{
				"host": "db.example.com:5432",
			},
			expected: "db.example.com:5432",
		},
		{
			name:       "DNS with host",
			pluginName: "dns",
			config: map[string]interface{}{
				"host": "example.com",
			},
			expected: "example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := NewCapabilityAnalyzer(setupTestRegistry())

			profile := &entities.Profile{
				Metadata: entities.ProfileMetadata{Name: "test", Version: "1.0.0"},
				Controls: entities.ControlsSection{
					Items: []entities.Control{
						{
							ID: "net-check",
							ObservationDefinitions: []entities.ObservationDefinition{
								{
									Plugin: tt.pluginName,
									Config: tt.config,
								},
							},
						},
					},
				},
			}

			caps := analyzer.ExtractCapabilities(profile)

			assert.Contains(t, caps, tt.pluginName)
			require.NotNil(t, caps[tt.pluginName])
			require.NotNil(t, caps[tt.pluginName].Network)
			require.Len(t, caps[tt.pluginName].Network.Rules, 1)
			assert.Contains(t, caps[tt.pluginName].Network.Rules[0].Hosts, tt.expected)
		})
	}
}

func TestCapabilityAnalyzer_ExtractCapabilities_Deduplication(t *testing.T) {
	analyzer := NewCapabilityAnalyzer(setupTestRegistry())

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "test", Version: "1.0.0"},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID: "check1",
					ObservationDefinitions: []entities.ObservationDefinition{
						{
							Plugin: "file",
							Config: map[string]interface{}{"path": "/etc/passwd"},
						},
					},
				},
				{
					ID: "check2",
					ObservationDefinitions: []entities.ObservationDefinition{
						{
							Plugin: "file",
							Config: map[string]interface{}{"path": "/etc/passwd"}, // Duplicate
						},
					},
				},
				{
					ID: "check3",
					ObservationDefinitions: []entities.ObservationDefinition{
						{
							Plugin: "file",
							Config: map[string]interface{}{"path": "/etc/shadow"}, // Different
						},
					},
				},
			},
		},
	}

	caps := analyzer.ExtractCapabilities(profile)

	// Should have merged all paths
	assert.Contains(t, caps, "file")
	require.NotNil(t, caps["file"])
	require.NotNil(t, caps["file"].FS)

	// GrantSet.Merge appends rules, so we'll have multiple rules
	// Check that both paths are represented
	var allReadPaths []string
	for _, rule := range caps["file"].FS.Rules {
		allReadPaths = append(allReadPaths, rule.Read...)
	}
	assert.Contains(t, allReadPaths, "/etc/passwd")
	assert.Contains(t, allReadPaths, "/etc/shadow")
}

func TestCapabilityAnalyzer_ExtractCapabilities_MultiplePlugins(t *testing.T) {
	analyzer := NewCapabilityAnalyzer(setupTestRegistry())

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "test", Version: "1.0.0"},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID: "mixed-check",
					ObservationDefinitions: []entities.ObservationDefinition{
						{
							Plugin: "file",
							Config: map[string]interface{}{"path": "/etc/passwd"},
						},
						{
							Plugin: "command",
							Config: map[string]interface{}{"command": "/bin/ls"},
						},
						{
							Plugin: "http",
							Config: map[string]interface{}{"url": "https://api.example.com"},
						},
					},
				},
			},
		},
	}

	caps := analyzer.ExtractCapabilities(profile)

	// Should have capabilities for all three plugins
	assert.Len(t, caps, 3)
	assert.Contains(t, caps, "file")
	assert.Contains(t, caps, "command")
	assert.Contains(t, caps, "http")
}

func TestCapabilityAnalyzer_ExtractCapabilities_NoExtractableCapabilities(t *testing.T) {
	analyzer := NewCapabilityAnalyzer(setupTestRegistry())

	profile := &entities.Profile{
		Metadata: entities.ProfileMetadata{Name: "test", Version: "1.0.0"},
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID: "unknown-plugin",
					ObservationDefinitions: []entities.ObservationDefinition{
						{
							Plugin: "unknown",
							Config: map[string]interface{}{"foo": "bar"},
						},
					},
				},
			},
		},
	}

	caps := analyzer.ExtractCapabilities(profile)

	// Should return empty map for unknown plugins
	assert.Empty(t, caps)
}

func TestCapabilityAnalyzer_ExtractCapabilities_InvalidConfig(t *testing.T) {
	tests := []struct {
		name       string
		pluginName string
		config     map[string]interface{}
	}{
		{
			name:       "File with empty path",
			pluginName: "file",
			config:     map[string]interface{}{"path": ""},
		},
		{
			name:       "File with non-string path",
			pluginName: "file",
			config:     map[string]interface{}{"path": 123},
		},
		{
			name:       "Command with missing command",
			pluginName: "command",
			config:     map[string]interface{}{"args": []string{"foo"}},
		},
		{
			name:       "HTTP with neither url nor host",
			pluginName: "http",
			config:     map[string]interface{}{"method": "GET"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := NewCapabilityAnalyzer(setupTestRegistry())

			profile := &entities.Profile{
				Metadata: entities.ProfileMetadata{Name: "test", Version: "1.0.0"},
				Controls: entities.ControlsSection{
					Items: []entities.Control{
						{
							ID: "check",
							ObservationDefinitions: []entities.ObservationDefinition{
								{
									Plugin: tt.pluginName,
									Config: tt.config,
								},
							},
						},
					},
				},
			}

			caps := analyzer.ExtractCapabilities(profile)

			// Should gracefully handle invalid configs by not extracting capabilities
			assert.Empty(t, caps)
		})
	}
}
