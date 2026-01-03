// Package capabilities defines domain types for capability management.
package capabilities

import (
	"fmt"
	"os"
	"path/filepath"
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
func (p *Policy) IsGranted(request Capability, granted []Capability) bool {
	for _, grant := range granted {
		if grant.Kind != request.Kind {
			continue
		}

		var matches bool
		switch request.Kind {
		case "network":
			matches = matchNetworkPattern(request.Pattern, grant.Pattern)
		case "fs":
			matches = matchFilesystemPattern(request.Pattern, grant.Pattern)
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

func matchPortRange(port, portRange string) bool {
	rangeParts := strings.SplitN(portRange, "-", 2)
	if len(rangeParts) != 2 {
		return false
	}
	reqPortNum := 0
	if _, err := fmt.Sscanf(port, "%d", &reqPortNum); err != nil {
		return false
	}
	startPort := 0
	if _, err := fmt.Sscanf(strings.TrimSpace(rangeParts[0]), "%d", &startPort); err != nil {
		return false
	}
	endPort := 0
	if _, err := fmt.Sscanf(strings.TrimSpace(rangeParts[1]), "%d", &endPort); err != nil {
		return false
	}
	if reqPortNum < 1 || reqPortNum > 65535 || startPort < 1 || startPort > 65535 || endPort < 1 || endPort > 65535 || startPort > endPort {
		return false
	}
	return reqPortNum >= startPort && reqPortNum <= endPort
}

func matchFilesystemPattern(requested, granted string) bool {
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
		cwd, err := os.Getwd()
		if err != nil {
			return false
		}
		reqPath = filepath.Join(cwd, reqPath)
		reqPath = filepath.Clean(reqPath)
	}

	realPath, err := filepath.EvalSymlinks(reqPath)
	if err == nil {
		reqPath = realPath
	}

	if !filepath.IsAbs(grantPattern) && !strings.Contains(grantPattern, "**") {
		cwd, err := os.Getwd()
		if err != nil {
			return false
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
