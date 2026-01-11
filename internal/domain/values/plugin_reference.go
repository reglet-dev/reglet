package values

import (
	"fmt"
	"strings"
)

// PluginReference uniquely identifies a plugin version.
// Format: registry.io/org/repo/name:version or name (for embedded)
type PluginReference struct {
	registry string // ghcr.io
	org      string // whiskeyjimbo
	repo     string // reglet-plugins
	name     string // file
	version  string // 1.0.2
}

// NewPluginReference creates a reference from components.
func NewPluginReference(registry, org, repo, name, version string) PluginReference {
	return PluginReference{
		registry: registry,
		org:      org,
		repo:     repo,
		name:     name,
		version:  version,
	}
}

// ParsePluginReference parses OCI reference string.
// Examples:
//   - file (embedded)
//   - ghcr.io/whiskeyjimbo/reglet-plugins/file:1.0.2
func ParsePluginReference(ref string) (PluginReference, error) {
	// Embedded plugin (simple name)
	if !strings.Contains(ref, "/") && !strings.Contains(ref, ":") {
		return PluginReference{name: ref}, nil
	}

	// OCI reference: registry.io/org/repo/name:version
	parts := strings.Split(ref, "/")
	if len(parts) < 4 {
		return PluginReference{}, fmt.Errorf("invalid OCI reference: %s", ref)
	}

	nameVersion := strings.Split(parts[len(parts)-1], ":")
	if len(nameVersion) != 2 {
		return PluginReference{}, fmt.Errorf("missing version tag: %s", ref)
	}

	return PluginReference{
		registry: parts[0],
		org:      parts[1],
		repo:     parts[2],
		name:     nameVersion[0],
		version:  nameVersion[1],
	}, nil
}

// String returns the canonical OCI reference string.
func (r PluginReference) String() string {
	if r.IsEmbedded() {
		return r.name
	}
	return fmt.Sprintf("%s/%s/%s/%s:%s",
		r.registry, r.org, r.repo, r.name, r.version)
}

// IsEmbedded returns true if this is a built-in plugin.
func (r PluginReference) IsEmbedded() bool {
	return r.registry == ""
}

// Name returns the plugin name.
func (r PluginReference) Name() string {
	return r.name
}

// Version returns the version tag.
func (r PluginReference) Version() string {
	return r.version
}

// Registry returns the registry hostname.
func (r PluginReference) Registry() string {
	return r.registry
}

// Equals checks equality with another reference.
func (r PluginReference) Equals(other PluginReference) bool {
	return r.registry == other.registry &&
		r.org == other.org &&
		r.repo == other.repo &&
		r.name == other.name &&
		r.version == other.version
}
