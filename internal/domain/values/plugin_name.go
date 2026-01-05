package values

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PluginName represents a validated plugin identifier.
// Enforces non-empty, trimmed plugin names.
type PluginName struct {
	value string
}

// NewPluginName creates a PluginName with strict validation.
// A valid plugin name must:
// - Be non-empty
// - contain only alphanumeric characters, underscores, and hyphens
// - NOT contain paths, dots, or special characters
// - Be at most 64 characters long
func NewPluginName(name string) (PluginName, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return PluginName{}, fmt.Errorf("plugin name cannot be empty")
	}

	if len(name) > 64 {
		return PluginName{}, fmt.Errorf("plugin name too long (max 64 chars)")
	}

	// Security check: Path separators
	if strings.ContainsAny(name, `/\`) {
		return PluginName{}, fmt.Errorf("plugin name cannot contain path separators")
	}

	// Security check: Directory traversal
	if strings.Contains(name, "..") {
		return PluginName{}, fmt.Errorf("plugin name cannot contain parent directory references")
	}

	// Validate characters: simple alphanumeric + underscore + hyphen
	// No slashes, dots, or other special characters allowed
	for _, ch := range name {
		if !isValidPluginChar(ch) {
			return PluginName{}, fmt.Errorf("invalid plugin name %q: must contain only alphanumeric characters, underscores, and hyphens", name)
		}
	}

	return PluginName{value: name}, nil
}

func isValidPluginChar(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		r == '_' ||
		r == '-'
}

// MustNewPluginName creates a PluginName or panics
func MustNewPluginName(name string) PluginName {
	pn, err := NewPluginName(name)
	if err != nil {
		panic(err)
	}
	return pn
}

// String returns the string representation
func (p PluginName) String() string {
	return p.value
}

// IsEmpty returns true if this is the zero value
func (p PluginName) IsEmpty() bool {
	return p.value == ""
}

// Equals checks if two plugin names are equal
func (p PluginName) Equals(other PluginName) bool {
	return p.value == other.value
}

// MarshalJSON implements json.Marshaler.
// Uses json.Marshal for proper character escaping.
func (p PluginName) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.value)
}

// UnmarshalJSON implements json.Unmarshaler
func (p *PluginName) UnmarshalJSON(data []byte) error {
	s := string(data)
	if len(s) < 2 {
		return fmt.Errorf("invalid plugin name JSON")
	}
	s = s[1 : len(s)-1]

	name, err := NewPluginName(s)
	if err != nil {
		return err
	}
	*p = name
	return nil
}
