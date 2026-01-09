package services

import (
	"testing"

	"github.com/reglet-dev/reglet/internal/domain/capabilities"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/stretchr/testify/assert"
)

// Mock extractors for testing (replicates logic that moved to infrastructure)
type testFileExtractor struct{}

func (e *testFileExtractor) Extract(config map[string]interface{}) []capabilities.Capability {
	var caps []capabilities.Capability
	if pathVal, ok := config["path"]; ok {
		if path, ok := pathVal.(string); ok && path != "" {
			caps = append(caps, capabilities.Capability{Kind: "fs", Pattern: "read:" + path})
		}
	}
	return caps
}

type testCommandExtractor struct{}

func (e *testCommandExtractor) Extract(config map[string]interface{}) []capabilities.Capability {
	var caps []capabilities.Capability
	if cmdVal, ok := config["command"]; ok {
		if cmd, ok := cmdVal.(string); ok && cmd != "" {
			caps = append(caps, capabilities.Capability{Kind: "exec", Pattern: cmd})
		}
	}
	return caps
}

type testNetworkExtractor struct{}

func (e *testNetworkExtractor) Extract(config map[string]interface{}) []capabilities.Capability {
	var caps []capabilities.Capability
	if urlVal, ok := config["url"]; ok {
		if url, ok := urlVal.(string); ok && url != "" {
			caps = append(caps, capabilities.Capability{Kind: "network", Pattern: "outbound:" + url})
		}
	}
	if hostVal, ok := config["host"]; ok {
		if host, ok := hostVal.(string); ok && host != "" {
			caps = append(caps, capabilities.Capability{Kind: "network", Pattern: "outbound:" + host})
		}
	}
	return caps
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
	assert.Len(t, caps["file"], 1)
	assert.Equal(t, "fs", caps["file"][0].Kind)
	assert.Equal(t, "read:/etc/passwd", caps["file"][0].Pattern)
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
	assert.Len(t, caps["command"], 1)
	assert.Equal(t, "exec", caps["command"][0].Kind)
	assert.Equal(t, "/usr/bin/systemctl", caps["command"][0].Pattern)
}

func TestCapabilityAnalyzer_ExtractCapabilities_NetworkPlugins(t *testing.T) {
	tests := []struct {
		name       string
		pluginName string
		config     map[string]interface{}
		expected   capabilities.Capability
	}{
		{
			name:       "HTTP with URL",
			pluginName: "http",
			config: map[string]interface{}{
				"url": "https://api.example.com",
			},
			expected: capabilities.Capability{
				Kind:    "network",
				Pattern: "outbound:https://api.example.com",
			},
		},
		{
			name:       "TCP with host",
			pluginName: "tcp",
			config: map[string]interface{}{
				"host": "db.example.com:5432",
			},
			expected: capabilities.Capability{
				Kind:    "network",
				Pattern: "outbound:db.example.com:5432",
			},
		},
		{
			name:       "DNS with host",
			pluginName: "dns",
			config: map[string]interface{}{
				"host": "example.com",
			},
			expected: capabilities.Capability{
				Kind:    "network",
				Pattern: "outbound:example.com",
			},
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
			assert.Len(t, caps[tt.pluginName], 1)
			assert.Equal(t, tt.expected, caps[tt.pluginName][0])
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

	// Should deduplicate /etc/passwd but keep /etc/shadow
	assert.Contains(t, caps, "file")
	assert.Len(t, caps["file"], 2)

	patterns := make(map[string]bool)
	for _, cap := range caps["file"] {
		patterns[cap.Pattern] = true
	}
	assert.True(t, patterns["read:/etc/passwd"])
	assert.True(t, patterns["read:/etc/shadow"])
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
