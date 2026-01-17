package scaffold

// GetCapabilitiesForPlugins returns all capability grants needed for the given plugins.
// This aggregates capabilities from all plugin examples.
func GetCapabilitiesForPlugins(plugins []string) []CapabilityGrant {
	// Use a map to deduplicate capabilities
	seen := make(map[string]bool)
	var capabilities []CapabilityGrant

	for _, name := range plugins {
		if example := GetPluginExample(name); example != nil {
			for _, cap := range example.Capabilities {
				key := cap.Kind + ":" + cap.Pattern
				if !seen[key] {
					seen[key] = true
					capabilities = append(capabilities, cap)
				}
			}
		}
	}

	return capabilities
}
