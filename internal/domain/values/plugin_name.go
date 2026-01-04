package values

import (
	"fmt"
	"strings"
)

// PluginName represents a validated plugin identifier.
// Enforces non-empty, trimmed plugin names.
type PluginName struct {
	value string
}

// NewPluginName creates a PluginName with validation
func NewPluginName(name string) (PluginName, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return PluginName{}, fmt.Errorf("plugin name cannot be empty")
	}
	return PluginName{value: name}, nil
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

// MarshalJSON implements json.Marshaler
func (p PluginName) MarshalJSON() ([]byte, error) {
	return []byte(`"` + p.value + `"`), nil
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
