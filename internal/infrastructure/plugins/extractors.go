// Package plugins provides infrastructure implementations for plugin capabilities.
package plugins

import (
	"github.com/whiskeyjimbo/reglet/internal/domain/capabilities"
)

// FileExtractor extracts filesystem capabilities.
type FileExtractor struct{}

// Extract analyzes observation config and returns required filesystem capabilities.
func (e *FileExtractor) Extract(config map[string]interface{}) []capabilities.Capability {
	var caps []capabilities.Capability
	if pathVal, ok := config["path"]; ok {
		if path, ok := pathVal.(string); ok && path != "" {
			caps = append(caps, capabilities.Capability{
				Kind:    "fs",
				Pattern: "read:" + path,
			})
		}
	}
	return caps
}

// CommandExtractor extracts execution capabilities.
type CommandExtractor struct{}

// Extract analyzes observation config and returns required execution capabilities.
func (e *CommandExtractor) Extract(config map[string]interface{}) []capabilities.Capability {
	var caps []capabilities.Capability
	if cmdVal, ok := config["command"]; ok {
		if cmd, ok := cmdVal.(string); ok && cmd != "" {
			caps = append(caps, capabilities.Capability{
				Kind:    "exec",
				Pattern: cmd,
			})
		}
	}
	return caps
}

// NetworkExtractor extracts network capabilities.
type NetworkExtractor struct{}

// Extract analyzes observation config and returns required network capabilities.
func (e *NetworkExtractor) Extract(config map[string]interface{}) []capabilities.Capability {
	var caps []capabilities.Capability

	// Check for "url" (http)
	if urlVal, ok := config["url"]; ok {
		if url, ok := urlVal.(string); ok && url != "" {
			caps = append(caps, capabilities.Capability{
				Kind:    "network",
				Pattern: "outbound:" + url,
			})
		}
	}

	// Check for "host" (tcp, dns) - only if url wasn't found or to support both
	if hostVal, ok := config["host"]; ok {
		if host, ok := hostVal.(string); ok && host != "" {
			// Deduplicate if needed, but for now just add it.
			// The analyzer usually deduplicates, but let's be safe.
			// If we already added a capability for this pattern (from url), we might duplicate.
			// But the core analyzer does deduplication.
			caps = append(caps, capabilities.Capability{
				Kind:    "network",
				Pattern: "outbound:" + host,
			})
		}
	}

	return caps
}

// RegisterDefaultExtractors registers the built-in plugin extractors.
func RegisterDefaultExtractors(registry *capabilities.Registry) {
	registry.Register("file", &FileExtractor{})
	registry.Register("command", &CommandExtractor{})

	netExtractor := &NetworkExtractor{}
	registry.Register("http", netExtractor)
	registry.Register("tcp", netExtractor)
	registry.Register("dns", netExtractor)
}
