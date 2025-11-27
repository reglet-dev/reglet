// Package hostfuncs provides host functions for WASM plugins
package hostfuncs

// Capability represents a permission requirement
// Defined here to avoid import cycle with internal/wasm
type Capability struct {
	Kind    string // fs, network, env, exec
	Pattern string // e.g., "/etc/**", "80,443", "AWS_*"
}

// CapabilityChecker checks if operations are allowed based on granted capabilities
type CapabilityChecker struct {
	grantedCapabilities []Capability
}

// NewCapabilityChecker creates a new capability checker with the given capabilities
func NewCapabilityChecker(caps []Capability) *CapabilityChecker {
	return &CapabilityChecker{
		grantedCapabilities: caps,
	}
}
