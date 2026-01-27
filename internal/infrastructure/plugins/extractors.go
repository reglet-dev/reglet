// Package plugins provides infrastructure implementations for plugin capabilities.
package plugins

import (
	"strconv"

	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/capabilities"
)

// FileExtractor extracts filesystem capabilities.
type FileExtractor struct{}

// Extract analyzes observation config and returns required filesystem capabilities.
func (e *FileExtractor) Extract(config map[string]interface{}) *entities.GrantSet {
	if pathVal, ok := config["path"]; ok {
		if path, ok := pathVal.(string); ok && path != "" {
			return &entities.GrantSet{
				FS: &entities.FileSystemCapability{
					Rules: []entities.FileSystemRule{
						{Read: []string{path}},
					},
				},
			}
		}
	}
	return nil
}

// CommandExtractor extracts execution capabilities.
type CommandExtractor struct{}

// Extract analyzes observation config and returns required execution capabilities.
func (e *CommandExtractor) Extract(config map[string]interface{}) *entities.GrantSet {
	if cmdVal, ok := config["command"]; ok {
		if cmd, ok := cmdVal.(string); ok && cmd != "" {
			return &entities.GrantSet{
				Exec: &entities.ExecCapability{
					Commands: []string{cmd},
				},
			}
		}
	}
	return nil
}

// NetworkExtractor extracts network capabilities.
type NetworkExtractor struct{}

// Extract analyzes observation config and returns required network capabilities.
func (e *NetworkExtractor) Extract(config map[string]interface{}) *entities.GrantSet {
	var ports []string

	// Check for "url" (http) - extract port or use default
	if urlVal, ok := config["url"]; ok {
		if url, ok := urlVal.(string); ok && url != "" {
			// For HTTP URLs, default to port 443 (https) or 80 (http)
			ports = append(ports, "443", "80")
		}
	}

	// Check for "host" (tcp, dns)
	if _, ok := config["host"]; ok {
		// Host alone doesn't determine port
	}

	// Check for "port" (tcp)
	if portVal, ok := config["port"]; ok {
		var portStr string
		switch v := portVal.(type) {
		case string:
			portStr = v
		case float64:
			portStr = strconv.FormatFloat(v, 'f', 0, 64)
		case int:
			portStr = strconv.Itoa(v)
		}

		if portStr != "" {
			ports = append(ports, portStr)
		}
	}

	if len(ports) == 0 {
		return nil
	}

	return &entities.GrantSet{
		Network: &entities.NetworkCapability{
			Rules: []entities.NetworkRule{
				{Hosts: []string{"*"}, Ports: ports},
			},
		},
	}
}

// Ensure extractors implement the interface.
var (
	_ capabilities.Extractor = (*FileExtractor)(nil)
	_ capabilities.Extractor = (*CommandExtractor)(nil)
	_ capabilities.Extractor = (*NetworkExtractor)(nil)
)

// RegisterDefaultExtractors registers the built-in plugin extractors.
func RegisterDefaultExtractors(registry *capabilities.Registry) {
	registry.Register("file", &FileExtractor{})
	registry.Register("command", &CommandExtractor{})

	netExtractor := &NetworkExtractor{}
	registry.Register("http", netExtractor)
	registry.Register("tcp", netExtractor)
	registry.Register("dns", netExtractor)
}
