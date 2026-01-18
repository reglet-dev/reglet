// Package config provides configuration parsing and variable handling.
package config

import (
	"regexp"
	"strings"
)

// varReferencePattern matches {{ .vars.key }} or {{ .vars.key.nested }} patterns.
var varReferencePattern = regexp.MustCompile(`\{\{\s*\.vars\.([a-zA-Z0-9_]+(?:\.[a-zA-Z0-9_]+)*)\s*\}\}`)

// FindUnusedVars compares CLI variable keys against variables referenced in content.
// Returns a list of CLI variable keys that were set but not referenced.
// This helps users catch typos and misconfiguration.
func FindUnusedVars(cliVars map[string]interface{}, content string) []string {
	if len(cliVars) == 0 {
		return nil
	}

	// Extract all variable references from content
	referencedVars := extractVarReferences(content)

	// Build set of top-level CLI var keys
	cliKeys := extractTopLevelKeys(cliVars)

	// Find keys that are set but not referenced
	var unused []string
	for key := range cliKeys {
		if !isKeyReferenced(key, referencedVars) {
			unused = append(unused, key)
		}
	}

	return unused
}

// extractVarReferences finds all {{ .vars.X }} patterns in content.
func extractVarReferences(content string) map[string]bool {
	refs := make(map[string]bool)
	matches := varReferencePattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) >= 2 {
			// Extract the full path and just the top-level key
			fullPath := match[1]
			refs[fullPath] = true

			// Also add top-level key for nested paths
			if idx := strings.Index(fullPath, "."); idx != -1 {
				refs[fullPath[:idx]] = true
			}
		}
	}
	return refs
}

// extractTopLevelKeys gets all top-level keys from a nested map.
func extractTopLevelKeys(m map[string]interface{}) map[string]bool {
	keys := make(map[string]bool)
	for k := range m {
		keys[k] = true
	}
	return keys
}

// isKeyReferenced checks if a key or any of its nested paths are referenced.
func isKeyReferenced(key string, refs map[string]bool) bool {
	// Direct match
	if refs[key] {
		return true
	}

	// Check if any referenced path starts with this key
	for ref := range refs {
		if ref == key || strings.HasPrefix(ref, key+".") {
			return true
		}
	}

	return false
}

// FindUnusedVarsInProfile scans a profile's string content for variable references.
// The profile should be serialized to YAML or the raw YAML content.
func FindUnusedVarsInProfile(cliVars map[string]interface{}, profileContent []byte) []string {
	return FindUnusedVars(cliVars, string(profileContent))
}
