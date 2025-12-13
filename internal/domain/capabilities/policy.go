// Package capabilities defines domain types for capability management.
package capabilities

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
		if grant.Kind == request.Kind {
			// Basic pattern matching for now
			if matchPattern(request.Pattern, grant.Pattern) {
				return true
			}
		}
	}
	return false
}

// matchPattern performs simple glob-like pattern matching.
// Supports "*" wildcard at the end of the pattern.
func matchPattern(request, pattern string) bool {
	if pattern == "*" {
		return true // Universal wildcard
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(request, prefix)
	}
	return request == pattern
}
