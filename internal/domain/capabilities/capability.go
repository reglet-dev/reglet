// Package capabilities defines domain types for capability management.
package capabilities

// Capability represents a permission requirement or grant.
// This is a pure value object in the domain.
type Capability struct {
	Kind    string // fs, network, env, exec
	Pattern string // e.g., "/etc/**", "80,443", "AWS_*"
}
