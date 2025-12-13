// Package capabilities defines domain types for capability management.
package capabilities

// Grant represents a collection of capabilities granted to a plugin.
// This acts as a domain entity for managing approved permissions.
type Grant []Capability

// NewGrant creates a new empty Grant.
func NewGrant() Grant {
	return make(Grant, 0)
}

// Add adds a capability to the grant if it's not already present.
func (g *Grant) Add(cap Capability) {
	for _, existing := range *g {
		if existing.Equals(cap) {
			return // Already exists
		}
	}
	*g = append(*g, cap)
}

// Contains checks if the grant contains a specific capability.
func (g Grant) Contains(cap Capability) bool {
	for _, existing := range g {
		if existing.Equals(cap) {
			return true
		}
	}
	return false
}

// ContainsAny checks if the grant contains any of the given capabilities.
func (g Grant) ContainsAny(caps []Capability) bool {
	for _, cap := range caps {
		if g.Contains(cap) {
			return true
		}
	}
	return false
}

// Remove removes a capability from the grant.
func (g *Grant) Remove(cap Capability) {
	for i, existing := range *g {
		if existing.Equals(cap) {
			*g = append((*g)[:i], (*g)[i+1:]...)
			return
		}
	}
}
