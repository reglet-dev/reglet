// Package entities contains domain entities for the Reglet domain model.
package entities

import (
	"fmt"
	"strings"
)

// PluginSpec represents a plugin declaration with optional version and source.
type PluginSpec struct {
	// Name is the alias used in observations (e.g., "file", "file-legacy")
	Name string

	// Source is the plugin source (e.g., "file", "ghcr.io/reglet-dev/reglet-plugins/file:1.0.0")
	Source string

	// Version is the explicit version constraint (e.g., "1.2.0")
	Version string

	// Digest is the optional content hash for pinning (e.g., "sha256:abc123...")
	Digest string

	// Verify indicates whether signature verification is required
	Verify bool
}

// IsBuiltIn returns true if this plugin references a built-in plugin.
func (ps *PluginSpec) IsBuiltIn() bool {
	// Built-in plugins have simple names without registry prefixes
	return !strings.Contains(ps.Source, "/") && !strings.Contains(ps.Source, ":")
}

// PluginName returns the actual plugin name to load (without version suffix).
func (ps *PluginSpec) PluginName() string {
	// Extract base name from source
	source := ps.Source

	// Remove digest if present (e.g., "file@sha256:abc" -> "file")
	if idx := strings.Index(source, "@sha256:"); idx != -1 {
		source = source[:idx]
	}

	// Remove version if present (e.g., "file@1.0.0" -> "file", "file:1.0.0" -> "file")
	if idx := strings.LastIndex(source, "@"); idx != -1 {
		source = source[:idx]
	}
	if idx := strings.LastIndex(source, ":"); idx != -1 && !strings.Contains(source, "/") {
		// Only strip trailing version for simple names, not registry paths
		source = source[:idx]
	}

	// For OCI references, extract the plugin name from path
	if strings.Contains(source, "/") {
		parts := strings.Split(source, "/")
		source = parts[len(parts)-1]
		// Strip tag if present
		if idx := strings.LastIndex(source, ":"); idx != -1 {
			source = source[:idx]
		}
	}

	return source
}

// PluginRegistry maps plugin aliases to their specifications.
// This allows observations to reference plugins by alias while the runtime
// resolves them to their actual sources.
type PluginRegistry struct {
	specs map[string]*PluginSpec
}

// NewPluginRegistry creates a new empty plugin registry.
func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		specs: make(map[string]*PluginSpec),
	}
}

// Register adds a plugin specification to the registry.
func (pr *PluginRegistry) Register(spec *PluginSpec) error {
	if spec.Name == "" {
		return fmt.Errorf("plugin spec name cannot be empty")
	}
	if spec.Source == "" {
		return fmt.Errorf("plugin spec source cannot be empty for %q", spec.Name)
	}
	pr.specs[spec.Name] = spec
	return nil
}

// Resolve looks up a plugin by alias and returns its specification.
// If the alias is not registered, it returns a default spec where name=source.
func (pr *PluginRegistry) Resolve(alias string) *PluginSpec {
	if spec, ok := pr.specs[alias]; ok {
		return spec
	}
	// Return default spec for unregistered aliases (backwards compatibility)
	return &PluginSpec{
		Name:   alias,
		Source: alias,
	}
}

// HasPlugin reports whether a plugin with the given name is registered.
func (pr *PluginRegistry) HasPlugin(name string) bool {
	_, ok := pr.specs[name]
	return ok
}

// AllSpecs returns all registered plugin specifications.
func (pr *PluginRegistry) AllSpecs() []*PluginSpec {
	specs := make([]*PluginSpec, 0, len(pr.specs))
	for _, spec := range pr.specs {
		specs = append(specs, spec)
	}
	return specs
}

// ParsePluginDeclaration parses a single plugin declaration string.
// Supported formats:
//   - "file"                                    -> name=file, source=file
//   - "file@1.2.0"                              -> name=file, source=file, version=1.2.0
//   - "ghcr.io/.../file:1.2.0"                  -> name=file, source=full path
//   - "ghcr.io/.../file@sha256:abc..."          -> name=file, source=path, digest=sha256:abc...
func ParsePluginDeclaration(declaration string) (*PluginSpec, error) {
	if declaration == "" {
		return nil, fmt.Errorf("empty plugin declaration")
	}

	spec := &PluginSpec{
		Source: declaration,
	}

	// Check for digest pin
	if idx := strings.Index(declaration, "@sha256:"); idx != -1 {
		spec.Digest = declaration[idx+1:] // "sha256:abc..."
		declaration = declaration[:idx]
		spec.Source = declaration + "@" + spec.Digest
	}

	// Check for version suffix (simple name@version format or registry/name@version)
	// We already handled @sha256 above, so other @ is likely version.
	if idx := strings.LastIndex(declaration, "@"); idx != -1 {
		spec.Version = declaration[idx+1:]
		declaration = declaration[:idx]
	}

	// Determine name (alias) from the declaration
	if strings.Contains(declaration, "/") {
		// OCI reference: extract name from path
		parts := strings.Split(declaration, "/")
		name := parts[len(parts)-1]
		// Strip tag if present
		if idx := strings.LastIndex(name, ":"); idx != -1 {
			spec.Version = name[idx+1:]
			name = name[:idx]
		}
		spec.Name = name
	} else {
		// Simple name or name@version
		spec.Name = declaration
		if spec.Version != "" {
			spec.Source = declaration // Without version for loading
		}
	}

	return spec, nil
}

// ParsePluginDeclarationWithAlias parses a plugin declaration with an explicit alias.
// Format: "alias: source" or expanded map format.
func ParsePluginDeclarationWithAlias(alias string, source interface{}) (*PluginSpec, error) {
	if alias == "" {
		return nil, fmt.Errorf("plugin alias cannot be empty")
	}

	switch v := source.(type) {
	case string:
		// Simple format: "alias: source"
		spec, err := ParsePluginDeclaration(v)
		if err != nil {
			return nil, fmt.Errorf("invalid source for %q: %w", alias, err)
		}
		spec.Name = alias // Override name with explicit alias
		return spec, nil

	case map[string]interface{}:
		// Expanded format
		spec := &PluginSpec{
			Name: alias,
		}

		if src, ok := v["source"].(string); ok {
			spec.Source = src
		} else {
			return nil, fmt.Errorf("plugin %q: missing 'source' field", alias)
		}

		if digest, ok := v["digest"].(string); ok {
			spec.Digest = digest
		}

		if verify, ok := v["verify"].(bool); ok {
			spec.Verify = verify
		}

		return spec, nil

	default:
		return nil, fmt.Errorf("plugin %q: invalid source type %T", alias, source)
	}
}
