// Package plugins provides infrastructure implementations for plugin capabilities.
package plugins

import (
	"fmt"
	"strings"

	"github.com/reglet-dev/reglet/internal/domain/capabilities"
	"github.com/reglet-dev/reglet/internal/domain/capability"
)

// FileExtractor extracts required file system permissions.
type FileExtractor struct{}

func (e *FileExtractor) Extract(config map[string]interface{}) *capability.GrantSet {
	// Look for common file path fields in standard plugins
	// - file plugin: "path"
	path, ok := config["path"].(string)
	if !ok || path == "" {
		return nil
	}

	return &capability.GrantSet{
		FS: &capability.FileSystemCapability{
			Rules: []capability.FileSystemRule{
				{
					Read: []string{path},
				},
			},
		},
	}
}

// CommandExtractor extracts required exec permissions.
type CommandExtractor struct{}

func (e *CommandExtractor) Extract(config map[string]interface{}) *capability.GrantSet {
	// - command plugin: "command" or "cmd"
	cmd, ok := config["command"].(string)
	if !ok {
		cmd, ok = config["cmd"].(string)
	}
	if !ok || cmd == "" {
		return nil
	}

	return &capability.GrantSet{
		Exec: &capability.ExecCapability{
			Commands: []string{cmd},
		},
	}
}

// NetworkExtractor extracts required network permissions.
type NetworkExtractor struct{}

func (e *NetworkExtractor) Extract(config map[string]interface{}) *capability.GrantSet {
	var hosts []string
	var ports []string

	// HTTP URL
	if url, ok := config["url"].(string); ok && url != "" {
		if host := extractHostFromURL(url); host != "" {
			hosts = append(hosts, host)
			if strings.HasPrefix(url, "https://") {
				ports = append(ports, "443")
			} else if strings.HasPrefix(url, "http://") {
				ports = append(ports, "80")
			}
		}
	}

	// Host/Target field
	if host, ok := config["host"].(string); ok && host != "" {
		hosts = append(hosts, host)
	}
	if target, ok := config["target"].(string); ok && target != "" {
		hosts = append(hosts, target)
	}

	// Nameserver field (dns)
	if ns, ok := config["nameserver"].(string); ok && ns != "" {
		hosts = append(hosts, ns)
		ports = append(ports, "53")
	}

	// Port field
	// Port field
	if port, ok := config["port"]; ok {
		switch v := port.(type) {
		case int:
			if v > 0 {
				ports = append(ports, fmt.Sprintf("%d", v))
			}
		case string:
			if v != "" {
				ports = append(ports, v)
			}
		case float64:
			ports = append(ports, fmt.Sprintf("%.0f", v))
		case uint64:
			ports = append(ports, fmt.Sprintf("%d", v))
		case int64:
			ports = append(ports, fmt.Sprintf("%d", v))
		case int32:
			ports = append(ports, fmt.Sprintf("%d", v))
		}
	}

	if len(hosts) == 0 {
		if len(ports) > 0 {
			// If ports are specified but no host, assume wildcard host
			hosts = []string{"*"}
		} else {
			return nil
		}
	}

	// Default ports if not specified
	if len(ports) == 0 {
		// Default to wildcard for broad connectivity if host is specified but port is not
		ports = []string{"*"}
	}

	return &capability.GrantSet{
		Network: &capability.NetworkCapability{
			Rules: []capability.NetworkRule{
				{
					Hosts: hosts,
					Ports: ports,
				},
			},
		},
	}
}

func extractHostFromURL(url string) string {
	parts := strings.Split(url, "://")
	if len(parts) < 2 {
		return ""
	}
	remaining := parts[1]
	// Cut at first slash
	if idx := strings.Index(remaining, "/"); idx != -1 {
		remaining = remaining[:idx]
	}
	// Cut at port
	if idx := strings.Index(remaining, ":"); idx != -1 {
		remaining = remaining[:idx]
	}
	return remaining
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
	registry.Register("file.managed", &FileExtractor{}) // Alias
	registry.Register("command", &CommandExtractor{})

	netExtractor := &NetworkExtractor{}
	registry.Register("http", netExtractor)
	registry.Register("tcp", netExtractor)
	registry.Register("dns", netExtractor)
	registry.Register("smtp", netExtractor) // Add smtp
}
