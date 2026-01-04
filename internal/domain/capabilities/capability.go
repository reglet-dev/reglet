// Package capabilities defines domain types for capability management.
package capabilities

import "strings"

// Security risk assessment constants - domain knowledge about dangerous patterns
var (
	// Broad filesystem patterns that grant excessive access
	broadFilesystemPatterns = []string{
		"**", "/**", "read:**", "write:**", "read:/", "write:/",
		"read:/etc/**", "write:/etc/**",
		"read:/root/**", "write:/root/**",
		"read:/home/**", "write:/home/**",
	}

	// Shell interpreters that allow arbitrary command execution
	dangerousShells = []string{"bash", "sh", "zsh", "fish", "/bin/bash", "/bin/sh"}

	// Script interpreters that can execute arbitrary code via flags (-c, -e, etc.)
	// Matches base + versioned variants (python3, python3.11, etc.)
	dangerousInterpreters = []string{
		"python", "perl", "ruby", "node", "nodejs",
		"php", "lua", "awk", "gawk", "mawk", "nawk",
		"tclsh", "wish", "expect", "irb",
	}

	// Broad environment variable patterns
	broadEnvPatterns = []string{"*", "AWS_*", "AZURE_*", "GCP_*"}
)

// RiskLevel represents the security risk level of a capability.
type RiskLevel int

const (
	// RiskLevelLow represents minimal security risk (specific, narrow permissions).
	RiskLevelLow RiskLevel = iota
	// RiskLevelMedium represents moderate security risk (network access, read-only sensitive data).
	RiskLevelMedium
	// RiskLevelHigh represents high security risk (broad permissions, arbitrary code execution).
	RiskLevelHigh
)

// String returns a human-readable representation of the risk level.
func (r RiskLevel) String() string {
	switch r {
	case RiskLevelLow:
		return "low"
	case RiskLevelMedium:
		return "medium"
	case RiskLevelHigh:
		return "high"
	default:
		return "unknown"
	}
}

// Capability represents a permission requirement or grant.
// This is a pure value object in the domain.
type Capability struct {
	Kind    string // fs, network, env, exec
	Pattern string // e.g., "/etc/**", "80,443", "AWS_*"
}

// Equals checks if two capabilities are equal (value object equality).
func (c Capability) Equals(other Capability) bool {
	return c.Kind == other.Kind && c.Pattern == other.Pattern
}

// String returns a human-readable representation of the capability.
func (c Capability) String() string {
	return c.Kind + ":" + c.Pattern
}

// IsEmpty returns true if this is a zero-value capability.
func (c Capability) IsEmpty() bool {
	return c.Kind == "" && c.Pattern == ""
}

// IsBroad returns true if this capability pattern is overly permissive.
func (c Capability) IsBroad() bool {
	switch c.Kind {
	case "fs":
		return matchesAny(c.Pattern, broadFilesystemPatterns)

	case "exec":
		// Wildcard patterns
		if c.Pattern == "**" || c.Pattern == "*" {
			return true
		}
		// Shell or interpreter without specific script path
		return matchesAny(c.Pattern, dangerousShells) || matchesInterpreter(c.Pattern)

	case "network":
		return c.Pattern == "*" || c.Pattern == "outbound:*"

	case "env":
		return matchesAny(c.Pattern, broadEnvPatterns)

	default:
		return false
	}
}

// RiskLevel returns the security risk level of this capability.
// This is a core business rule that determines how capabilities are presented to users.
func (c Capability) RiskLevel() RiskLevel {
	if c.IsBroad() {
		return RiskLevelHigh
	}

	// Medium risk: network access or command execution (even if specific)
	if c.Kind == "network" || c.Kind == "exec" {
		return RiskLevelMedium
	}

	// Medium risk: reading sensitive system files
	if c.Kind == "fs" && strings.HasPrefix(c.Pattern, "read:/etc/") {
		return RiskLevelMedium
	}

	return RiskLevelLow
}

// RiskDescription returns a human-readable explanation of the security risk.
// This encapsulates domain knowledge about what each capability means.
func (c Capability) RiskDescription() string {
	switch c.Kind {
	case "fs":
		if strings.Contains(c.Pattern, "**") {
			return "Plugin can access ALL files on the system"
		}
		if strings.Contains(c.Pattern, "/etc") {
			return "Plugin can access sensitive system configuration"
		}
		if strings.Contains(c.Pattern, "/root") || strings.Contains(c.Pattern, "/home") {
			return "Plugin can access user home directories and private files"
		}
		if strings.HasPrefix(c.Pattern, "write:") {
			return "Plugin can modify files on disk"
		}
		return "Plugin can read specific files"

	case "exec":
		if matchesAny(c.Pattern, dangerousShells) {
			return "Plugin can execute arbitrary shell commands"
		}
		if matchesInterpreter(c.Pattern) {
			// Extract interpreter name (before : or version number)
			name := extractInterpreterName(c.Pattern)
			return "Plugin can execute arbitrary code via " + name + " interpreter"
		}
		return "Plugin can execute specific command: " + c.Pattern

	case "network":
		if c.Pattern == "*" || c.Pattern == "outbound:*" {
			return "Plugin can connect to any host on the internet"
		}
		return "Plugin can make network requests to: " + c.Pattern

	case "env":
		if c.Pattern == "*" {
			return `Grants access to ALL environment variables including:
    • Secrets and API keys from other tools
    • Shell configuration (PATH, HOME, etc.)
    • Potential credential leakage

Recommendation: Grant only specific variables:
    env:AWS_ACCESS_KEY_ID
    env:AWS_SECRET_ACCESS_KEY
    env:AWS_REGION`
		}
		if c.Pattern == "AWS_*" {
			return `Grants access to ALL AWS environment variables including:
    • AWS_ACCESS_KEY_ID (needed)
    • AWS_SECRET_ACCESS_KEY (needed)
    • AWS_SESSION_TOKEN (temporary credentials - high risk if leaked)

Recommendation: Grant only required variables individually`
		}
		return "Plugin can access environment variable: " + c.Pattern

	default:
		return "Plugin requires capability: " + c.String()
	}
}

// matchesAny checks if pattern exactly matches any string in the list
func matchesAny(pattern string, list []string) bool {
	for _, item := range list {
		if pattern == item {
			return true
		}
	}
	return false
}

// matchesInterpreter checks if pattern is a dangerous interpreter (base or versioned)
func matchesInterpreter(pattern string) bool {
	for _, base := range dangerousInterpreters {
		if isInterpreterVariant(pattern, base) {
			return true
		}
	}
	return false
}

// extractInterpreterName returns the base interpreter name from a pattern
// e.g., "python3.11" -> "python", "node:/script.js" -> "node"
func extractInterpreterName(pattern string) string {
	// Find first non-letter character
	for i, ch := range pattern {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')) {
			return pattern[:i]
		}
	}
	return pattern
}

// isInterpreterVariant checks if a pattern matches an interpreter base name or its versioned variants.
//
// Matches:
//   - Exact: "python"
//   - Versioned: "python3", "python3.11", "python2.7"
//   - Subpath: "python:*", "python:/path/to/script.py"
//
// Does NOT match:
//   - Unrelated: "pythonista", "python-config"
//   - Full paths: "/usr/bin/python" (handled elsewhere)
func isInterpreterVariant(pattern, baseInterpreter string) bool {
	if pattern == baseInterpreter {
		return true
	}

	if !strings.HasPrefix(pattern, baseInterpreter) {
		return false
	}

	// Check suffix is version number or subpath separator
	suffix := pattern[len(baseInterpreter):]
	if len(suffix) > 0 {
		first := suffix[0]
		// Version: python3, python3.11  OR  Subpath: python:*, python:/script.py
		return (first >= '0' && first <= '9') || first == '.' || first == ':'
	}

	return false
}
