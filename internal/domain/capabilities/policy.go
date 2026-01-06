// Package capabilities defines domain types for capability management.
package capabilities

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// Policy represents an authorization policy that determines if a requested operation is allowed.
// This is a pure domain service.
type Policy struct {
	// TODO: Implement more sophisticated policy logic
}

// NewPolicy creates a new domain policy.
func NewPolicy() *Policy {
	return &Policy{}
}

// IsGranted checks if a specific capability (request) is covered by any of the granted capabilities.
// The cwd parameter must be provided for filesystem capability checks that involve relative paths.
// Pass an empty string if filesystem checks are not needed or all paths are absolute.
func (p *Policy) IsGranted(request Capability, granted []Capability, cwd string) bool {
	for _, grant := range granted {
		if grant.Kind != request.Kind {
			continue
		}

		var matches bool
		switch request.Kind {
		case "network":
			matches = matchNetworkPattern(request.Pattern, grant.Pattern)
		case "fs":
			matches = matchFilesystemPattern(request.Pattern, grant.Pattern, cwd)
		case "env":
			matches = MatchEnvironmentPattern(request.Pattern, grant.Pattern)
		case "exec":
			matches = matchExecPattern(request.Pattern, grant.Pattern)
		default:
			// Fallback to simple equality or suffix wildcard for unknown kinds
			matches = matchPattern(request.Pattern, grant.Pattern)
		}

		if matches {
			return true
		}
	}
	return false
}

// matchPattern performs simple glob-like pattern matching.
func matchPattern(request, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(request, prefix)
	}
	return request == pattern
}

// matchNetworkPattern checks if a network request matches a granted pattern
func matchNetworkPattern(requested, granted string) bool {
	// Both should have format "outbound:<ports>" or "inbound:<ports>"
	reqParts := strings.SplitN(requested, ":", 2)
	grantParts := strings.SplitN(granted, ":", 2)

	if len(reqParts) != 2 || len(grantParts) != 2 {
		return false
	}

	if reqParts[0] != grantParts[0] {
		return false
	}

	reqPort := reqParts[1]
	grantPort := grantParts[1]

	if grantPort == "*" {
		return true
	}

	grantedPorts := strings.Split(grantPort, ",")
	for _, p := range grantedPorts {
		p = strings.TrimSpace(p)
		if strings.Contains(p, "-") {
			if matchPortRange(reqPort, p) {
				return true
			}
			continue
		}
		if p == reqPort {
			return true
		}
	}
	return false
}

// matchPortRange checks if a port falls within a port range (e.g., "80-443").
// Uses strict parsing - rejects malformed input like "80abc" or " 80".
func matchPortRange(port, portRange string) bool {
	// Parse port range "start-end"
	rangeParts := strings.SplitN(portRange, "-", 2)
	if len(rangeParts) != 2 {
		return false
	}

	// Parse requested port (must be pure numeric)
	reqPort, err := parsePort(port)
	if err != nil {
		return false
	}

	// Parse range bounds (allow whitespace around parts for flexibility)
	startPort, err := parsePort(strings.TrimSpace(rangeParts[0]))
	if err != nil {
		return false
	}

	endPort, err := parsePort(strings.TrimSpace(rangeParts[1]))
	if err != nil {
		return false
	}

	// Validate range semantics
	if startPort > endPort {
		return false
	}

	return reqPort >= startPort && reqPort <= endPort
}

// parsePort parses a port string and validates it's in the valid TCP/UDP range (1-65535).
// Rejects non-numeric input like "80abc" that fmt.Sscanf would partially parse.
func parsePort(s string) (int, error) {
	// strconv.Atoi is strict - "80abc" returns error, unlike fmt.Sscanf
	port, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port %d out of valid range 1-65535", port)
	}
	return port, nil
}

// matchFilesystemPattern checks if a filesystem request matches a granted pattern.
// The cwd parameter is used to resolve relative paths. If cwd is empty,
// relative paths will fail to match (defaulting to a safe deny).
func matchFilesystemPattern(requested, granted, cwd string) bool {
	reqParts := strings.SplitN(requested, ":", 2)
	grantParts := strings.SplitN(granted, ":", 2)

	if len(reqParts) != 2 || len(grantParts) != 2 {
		return false
	}
	if reqParts[0] != grantParts[0] {
		return false
	}

	reqPath := filepath.Clean(reqParts[1])
	grantPattern := grantParts[1]

	if !filepath.IsAbs(reqPath) {
		if cwd == "" {
			return false // No cwd provided, cannot resolve relative path
		}
		reqPath = filepath.Join(cwd, reqPath)
		reqPath = filepath.Clean(reqPath)
	}

	realPath, err := filepath.EvalSymlinks(reqPath)
	if err == nil {
		reqPath = realPath
	}

	if !filepath.IsAbs(grantPattern) && !strings.Contains(grantPattern, "**") {
		if cwd == "" {
			return false // No cwd provided, cannot resolve relative pattern
		}
		grantPattern = filepath.Join(cwd, grantPattern)
		grantPattern = filepath.Clean(grantPattern)
	}

	if strings.Contains(grantPattern, "**") {
		prefix := strings.TrimSuffix(grantPattern, "**")
		prefix = filepath.Clean(prefix) + string(filepath.Separator)
		return strings.HasPrefix(reqPath, prefix) || reqPath == strings.TrimSuffix(prefix, string(filepath.Separator))
	}

	matched, err := filepath.Match(grantPattern, reqPath)
	if err != nil {
		return false
	}
	return matched
}

// MatchEnvironmentPattern checks if an environment variable key matches a capability pattern.
// Supports exact match ("AWS_REGION"), prefix match ("AWS_*"), and wildcard ("*").
// This is the canonical implementation used by both capability enforcement and plugin injection.
//
// Examples:
//   - MatchEnvironmentPattern("AWS_REGION", "AWS_REGION") -> true (exact)
//   - MatchEnvironmentPattern("AWS_ACCESS_KEY_ID", "AWS_*") -> true (prefix)
//   - MatchEnvironmentPattern("PATH", "*") -> true (wildcard)
//   - MatchEnvironmentPattern("GCP_PROJECT", "AWS_*") -> false (no match)
func MatchEnvironmentPattern(requested, granted string) bool {
	// Wildcard matches everything (dangerous - should trigger warnings)
	if granted == "*" {
		return true
	}

	// Prefix match (e.g., "AWS_*" matches "AWS_ACCESS_KEY_ID", "AWS_REGION")
	if strings.HasSuffix(granted, "*") {
		prefix := strings.TrimSuffix(granted, "*")
		return strings.HasPrefix(requested, prefix)
	}

	// Exact match
	return requested == granted
}

func matchExecPattern(requested, granted string) bool {
	if granted == "**" {
		return true
	}
	if strings.HasSuffix(granted, "/*") {
		dir := strings.TrimSuffix(granted, "/*")
		reqDir := filepath.Dir(requested)
		return reqDir == dir
	}
	return requested == granted
}
