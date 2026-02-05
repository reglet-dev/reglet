// Package plugins provides infrastructure implementations for plugin capabilities.
package plugins

import (
	"net/url"
	"strconv"

	"github.com/reglet-dev/reglet-sdk/domain/entities"
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
	var hosts []string
	var ports []string

	e.extractFromURL(config, &hosts, &ports)
	e.extractFromPort(config, &hosts, &ports)
	e.extractFromNameserver(config, &hosts, &ports)

	if len(ports) == 0 || len(hosts) == 0 {
		return nil
	}

	return &entities.GrantSet{
		Network: &entities.NetworkCapability{
			Rules: []entities.NetworkRule{
				{Hosts: hosts, Ports: ports},
			},
		},
	}
}

// TODO: we probably shouldnt fall back to wildcard here, but it is better than nothing for now.
// extractFromURL extracts network capabilities from URL config (http/https).
func (e *NetworkExtractor) extractFromURL(config map[string]interface{}, hosts, ports *[]string) {
	urlVal, ok := config["url"]
	if !ok {
		return
	}

	urlStr, ok := urlVal.(string)
	if !ok || urlStr == "" {
		return
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil || parsedURL.Host == "" {
		// If URL parsing fails, fall back to wildcard for safety
		*hosts = append(*hosts, "*")
		*ports = append(*ports, "443", "80")
		return
	}

	// Extract hostname
	if hostname := parsedURL.Hostname(); hostname != "" {
		*hosts = append(*hosts, hostname)
	}

	// Select port based on scheme
	switch parsedURL.Scheme {
	case "https":
		*ports = append(*ports, "443")
	case "http":
		*ports = append(*ports, "80")
	default:
		// Unknown scheme, default to both
		*ports = append(*ports, "443", "80")
	}
}

// extractFromPort extracts network capabilities from port config (tcp).
func (e *NetworkExtractor) extractFromPort(config map[string]interface{}, hosts, ports *[]string) {
	portVal, ok := config["port"]
	if !ok {
		return
	}

	portStr := e.portToString(portVal)
	if portStr == "" {
		return
	}

	// Check if host is specified (for TCP)
	if hostVal, ok := config["host"]; ok {
		if hostStr, ok := hostVal.(string); ok && hostStr != "" {
			*hosts = append(*hosts, hostStr)
		} else {
			// Host field exists but is empty, use wildcard
			*hosts = append(*hosts, "*")
		}
	} else {
		// No host field, use wildcard for backward compatibility
		*hosts = append(*hosts, "*")
	}
	*ports = append(*ports, portStr)
}

// extractFromNameserver extracts network capabilities from nameserver config (dns).
func (e *NetworkExtractor) extractFromNameserver(config map[string]interface{}, hosts, ports *[]string) {
	nsVal, ok := config["nameserver"]
	if !ok {
		return
	}

	nsStr, ok := nsVal.(string)
	if !ok || nsStr == "" {
		return
	}

	*hosts = append(*hosts, nsStr)
	*ports = append(*ports, "53")
}

// portToString converts a port value to a string, handling multiple numeric types.
func (e *NetworkExtractor) portToString(portVal interface{}) string {
	switch v := portVal.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', 0, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	default:
		return ""
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
