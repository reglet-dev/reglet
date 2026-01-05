package capabilities

import (
	"sync"
)

// Extractor is an interface for extracting capabilities from a plugin configuration.
// Implementations of this interface contain plugin-specific logic for determining
// required permissions based on the user's configuration.
type Extractor interface {
	// Extract analyzes the configuration and returns a list of required capabilities.
	Extract(config map[string]interface{}) []Capability
}

// Registry manages the registration and retrieval of capability extractors.
type Registry struct {
	extractors map[string]Extractor
	mu         sync.RWMutex
}

// NewRegistry creates a new, empty capability registry.
func NewRegistry() *Registry {
	return &Registry{
		extractors: make(map[string]Extractor),
	}
}

// Register adds a capability extractor for a specific plugin.
// If an extractor allows overwriting, it will replace any existing one.
func (r *Registry) Register(pluginName string, extractor Extractor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.extractors[pluginName] = extractor
}

// Get retrieves the extractor for a given plugin.
// Returns nil and false if no extractor is registered.
func (r *Registry) Get(pluginName string) (Extractor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	extractor, ok := r.extractors[pluginName]
	return extractor, ok
}
