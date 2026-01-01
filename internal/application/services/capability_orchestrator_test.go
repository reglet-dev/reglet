package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
)

func TestExtractProfileCapabilities_FilePlugin(t *testing.T) {
	orchestrator := NewCapabilityOrchestrator(false)

	profile := &entities.Profile{
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "test-1",
					Name: "Test Control",
					Observations: []entities.Observation{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/etc/ssh/sshd_config",
							},
						},
					},
				},
			},
		},
	}

	caps := orchestrator.extractProfileCapabilities(profile)

	require.Contains(t, caps, "file")
	require.Len(t, caps["file"], 1)
	assert.Equal(t, "fs", caps["file"][0].Kind)
	assert.Equal(t, "read:/etc/ssh/sshd_config", caps["file"][0].Pattern)
}

func TestExtractProfileCapabilities_Deduplication(t *testing.T) {
	orchestrator := NewCapabilityOrchestrator(false)

	// Profile with 3 observations accessing the same file
	profile := &entities.Profile{
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "test-1",
					Name: "Test Control 1",
					Observations: []entities.Observation{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/etc/ssh/sshd_config",
							},
						},
					},
				},
				{
					ID:   "test-2",
					Name: "Test Control 2",
					Observations: []entities.Observation{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/etc/ssh/sshd_config",
							},
						},
					},
				},
				{
					ID:   "test-3",
					Name: "Test Control 3",
					Observations: []entities.Observation{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/etc/ssh/sshd_config",
							},
						},
					},
				},
			},
		},
	}

	caps := orchestrator.extractProfileCapabilities(profile)

	require.Contains(t, caps, "file")
	// Should deduplicate to 1 capability despite 3 observations
	require.Len(t, caps["file"], 1)
	assert.Equal(t, "fs", caps["file"][0].Kind)
	assert.Equal(t, "read:/etc/ssh/sshd_config", caps["file"][0].Pattern)
}

func TestExtractProfileCapabilities_MultipleFiles(t *testing.T) {
	orchestrator := NewCapabilityOrchestrator(false)

	profile := &entities.Profile{
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "test-1",
					Name: "Test Control",
					Observations: []entities.Observation{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/etc/ssh/sshd_config",
							},
						},
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/etc/hosts",
							},
						},
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/var/log/syslog",
							},
						},
					},
				},
			},
		},
	}

	caps := orchestrator.extractProfileCapabilities(profile)

	require.Contains(t, caps, "file")
	require.Len(t, caps["file"], 3)

	// Verify all three unique capabilities are present
	patterns := make(map[string]bool)
	for _, cap := range caps["file"] {
		assert.Equal(t, "fs", cap.Kind)
		patterns[cap.Pattern] = true
	}

	assert.True(t, patterns["read:/etc/ssh/sshd_config"])
	assert.True(t, patterns["read:/etc/hosts"])
	assert.True(t, patterns["read:/var/log/syslog"])
}

func TestExtractProfileCapabilities_CommandPlugin(t *testing.T) {
	orchestrator := NewCapabilityOrchestrator(false)

	profile := &entities.Profile{
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "test-1",
					Name: "Test Control",
					Observations: []entities.Observation{
						{
							Plugin: "command",
							Config: map[string]interface{}{
								"command": "systemctl status sshd",
							},
						},
					},
				},
			},
		},
	}

	caps := orchestrator.extractProfileCapabilities(profile)

	require.Contains(t, caps, "command")
	require.Len(t, caps["command"], 1)
	assert.Equal(t, "exec", caps["command"][0].Kind)
	assert.Equal(t, "systemctl status sshd", caps["command"][0].Pattern)
}

func TestExtractProfileCapabilities_NetworkPlugins(t *testing.T) {
	tests := []struct {
		name            string
		pluginName      string
		config          map[string]interface{}
		expectedKind    string
		expectedPattern string
	}{
		{
			name:            "HTTP with URL",
			pluginName:      "http",
			config:          map[string]interface{}{"url": "https://api.example.com"},
			expectedKind:    "network",
			expectedPattern: "outbound:https://api.example.com",
		},
		{
			name:            "TCP with host",
			pluginName:      "tcp",
			config:          map[string]interface{}{"host": "database.example.com:5432"},
			expectedKind:    "network",
			expectedPattern: "outbound:database.example.com:5432",
		},
		{
			name:            "DNS with host",
			pluginName:      "dns",
			config:          map[string]interface{}{"host": "example.com"},
			expectedKind:    "network",
			expectedPattern: "outbound:example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orchestrator := NewCapabilityOrchestrator(false)

			profile := &entities.Profile{
				Controls: entities.ControlsSection{
					Items: []entities.Control{
						{
							ID:   "test-1",
							Name: "Test Control",
							Observations: []entities.Observation{
								{
									Plugin: tt.pluginName,
									Config: tt.config,
								},
							},
						},
					},
				},
			}

			caps := orchestrator.extractProfileCapabilities(profile)

			require.Contains(t, caps, tt.pluginName)
			require.Len(t, caps[tt.pluginName], 1)
			assert.Equal(t, tt.expectedKind, caps[tt.pluginName][0].Kind)
			assert.Equal(t, tt.expectedPattern, caps[tt.pluginName][0].Pattern)
		})
	}
}

func TestExtractProfileCapabilities_EmptyConfig(t *testing.T) {
	orchestrator := NewCapabilityOrchestrator(false)

	// Profile with observation but no relevant config
	profile := &entities.Profile{
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "test-1",
					Name: "Test Control",
					Observations: []entities.Observation{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								// No "path" key
								"mode": "exists",
							},
						},
					},
				},
			},
		},
	}

	caps := orchestrator.extractProfileCapabilities(profile)

	// Should not extract any capabilities if config doesn't have required keys
	assert.Empty(t, caps)
}

func TestExtractProfileCapabilities_MultiplePlugins(t *testing.T) {
	orchestrator := NewCapabilityOrchestrator(false)

	profile := &entities.Profile{
		Controls: entities.ControlsSection{
			Items: []entities.Control{
				{
					ID:   "test-1",
					Name: "Test Control",
					Observations: []entities.Observation{
						{
							Plugin: "file",
							Config: map[string]interface{}{
								"path": "/etc/ssh/sshd_config",
							},
						},
						{
							Plugin: "command",
							Config: map[string]interface{}{
								"command": "systemctl status sshd",
							},
						},
						{
							Plugin: "http",
							Config: map[string]interface{}{
								"url": "https://api.example.com/health",
							},
						},
					},
				},
			},
		},
	}

	caps := orchestrator.extractProfileCapabilities(profile)

	// Should extract capabilities for all three plugins
	assert.Len(t, caps, 3)
	assert.Contains(t, caps, "file")
	assert.Contains(t, caps, "command")
	assert.Contains(t, caps, "http")

	assert.Equal(t, "fs", caps["file"][0].Kind)
	assert.Equal(t, "read:/etc/ssh/sshd_config", caps["file"][0].Pattern)

	assert.Equal(t, "exec", caps["command"][0].Kind)
	assert.Equal(t, "systemctl status sshd", caps["command"][0].Pattern)

	assert.Equal(t, "network", caps["http"][0].Kind)
	assert.Equal(t, "outbound:https://api.example.com/health", caps["http"][0].Pattern)
}
