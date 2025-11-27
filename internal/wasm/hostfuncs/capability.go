package hostfuncs

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Check verifies if the requested capability is granted
// Implements full pattern matching for network, fs, env, and exec capabilities
func (c *CapabilityChecker) Check(kind, pattern string) error {
	// Check if any granted capability matches the request
	for _, grant := range c.grantedCapabilities {
		if grant.Kind != kind {
			continue
		}

		// Pattern matching based on kind
		var matches bool
		switch kind {
		case "network":
			matches = matchNetworkPattern(pattern, grant.Pattern)
		case "fs":
			matches = matchFilesystemPattern(pattern, grant.Pattern)
		case "env":
			matches = matchEnvironmentPattern(pattern, grant.Pattern)
		case "exec":
			matches = matchExecPattern(pattern, grant.Pattern)
		default:
			// Unknown capability kind - deny by default
			continue
		}

		if matches {
			return nil
		}
	}

	// No matching grant found
	return fmt.Errorf("capability denied: %s:%s (no matching grant)", kind, pattern)
}

// matchNetworkPattern checks if a network request matches a granted pattern
// Supports:
//   - outbound:53 (exact port match)
//   - outbound:80,443 (port list)
//   - outbound:8000-9000 (port range)
//   - outbound:* (wildcard - any port)
func matchNetworkPattern(requested, granted string) bool {
	// Both should have format "outbound:<ports>" or "inbound:<ports>"
	reqParts := strings.SplitN(requested, ":", 2)
	grantParts := strings.SplitN(granted, ":", 2)

	if len(reqParts) != 2 || len(grantParts) != 2 {
		return false
	}

	// Direction must match (outbound vs inbound)
	if reqParts[0] != grantParts[0] {
		return false
	}

	reqPort := reqParts[1]
	grantPort := grantParts[1]

	// Wildcard grant allows any port
	if grantPort == "*" {
		return true
	}

	// Check if requested port is in the granted port list or range
	grantedPorts := strings.Split(grantPort, ",")
	for _, p := range grantedPorts {
		p = strings.TrimSpace(p)

		// Check for port range (e.g., "8000-9000")
		if strings.Contains(p, "-") {
			if matchPortRange(reqPort, p) {
				return true
			}
			continue
		}

		// Exact match
		if p == reqPort {
			return true
		}
	}

	return false
}

// matchPortRange checks if a port is within a granted range
// Range format: "8000-9000"
func matchPortRange(port, portRange string) bool {
	rangeParts := strings.SplitN(portRange, "-", 2)
	if len(rangeParts) != 2 {
		return false
	}

	// Parse requested port
	reqPortNum := 0
	if _, err := fmt.Sscanf(port, "%d", &reqPortNum); err != nil {
		return false
	}

	// Parse range start
	startPort := 0
	if _, err := fmt.Sscanf(strings.TrimSpace(rangeParts[0]), "%d", &startPort); err != nil {
		return false
	}

	// Parse range end
	endPort := 0
	if _, err := fmt.Sscanf(strings.TrimSpace(rangeParts[1]), "%d", &endPort); err != nil {
		return false
	}

	// Validate port numbers
	if reqPortNum < 1 || reqPortNum > 65535 {
		return false
	}
	if startPort < 1 || startPort > 65535 || endPort < 1 || endPort > 65535 {
		return false
	}
	if startPort > endPort {
		return false
	}

	// Check if port is in range
	return reqPortNum >= startPort && reqPortNum <= endPort
}

// matchFilesystemPattern checks if a filesystem request matches a granted pattern
// Supports glob patterns using filepath.Match
// Examples:
//   - read:/etc/** (any file under /etc)
//   - read:/var/log/*.log (log files only)
//   - write:/tmp/* (temp directory)
func matchFilesystemPattern(requested, granted string) bool {
	// Both should have format "read:<path>" or "write:<path>"
	reqParts := strings.SplitN(requested, ":", 2)
	grantParts := strings.SplitN(granted, ":", 2)

	if len(reqParts) != 2 || len(grantParts) != 2 {
		return false
	}

	// Operation must match (read vs write)
	if reqParts[0] != grantParts[0] {
		return false
	}

	reqPath := reqParts[1]
	grantPattern := grantParts[1]

	// Handle ** for recursive directory matching
	if strings.Contains(grantPattern, "**") {
		// Convert ** to a prefix match
		// e.g., "/etc/**" matches anything starting with "/etc/"
		prefix := strings.TrimSuffix(grantPattern, "**")
		prefix = strings.TrimSuffix(prefix, "/") + "/"
		return strings.HasPrefix(reqPath, prefix) || reqPath == strings.TrimSuffix(prefix, "/")
	}

	// Use filepath.Match for glob patterns
	matched, err := filepath.Match(grantPattern, reqPath)
	if err != nil {
		// Invalid pattern - deny by default
		return false
	}

	return matched
}

// matchEnvironmentPattern checks if an environment variable request matches a granted pattern
// Supports:
//   - AWS_* (wildcard prefix match)
//   - DB_PASSWORD (exact match)
func matchEnvironmentPattern(requested, granted string) bool {
	// Wildcard matching
	if strings.HasSuffix(granted, "*") {
		prefix := strings.TrimSuffix(granted, "*")
		return strings.HasPrefix(requested, prefix)
	}

	// Exact match
	return requested == granted
}

// matchExecPattern checks if an exec request matches a granted pattern
// Supports:
//   - /usr/bin/systemctl (exact binary path)
//   - /bin/* (any binary in directory)
func matchExecPattern(requested, granted string) bool {
	// Wildcard matching for directory
	if strings.HasSuffix(granted, "/*") {
		dir := strings.TrimSuffix(granted, "/*")
		reqDir := filepath.Dir(requested)
		return reqDir == dir
	}

	// Exact match
	return requested == granted
}
