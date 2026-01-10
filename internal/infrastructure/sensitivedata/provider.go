// Package sensitivedata provides tools for managing and protecting sensitive information
// such as secrets, passwords, and tokens.
package sensitivedata

import "sync"

// Provider implements ports.SensitiveValueProvider.
// It maintains a thread-safe registry of sensitive values.
type Provider struct {
	values []string
	mu     sync.RWMutex
}

// NewProvider creates a new sensitive data provider.
func NewProvider() *Provider {
	return &Provider{
		values: make([]string, 0, 32),
	}
}

// Track registers a sensitive value to be protected.
func (p *Provider) Track(value string) {
	if value == "" {
		return // Don't track empty values
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.values = append(p.values, value)
}

// AllValues returns all tracked sensitive values.
func (p *Provider) AllValues() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Return a copy to avoid race conditions if caller modifies the slice
	result := make([]string, len(p.values))
	copy(result, p.values)
	return result
}
