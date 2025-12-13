// Package capabilities defines domain types for capability management.
package capabilities

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
