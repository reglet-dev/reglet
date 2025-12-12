package valueobjects

import (
	"database/sql/driver"
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

// Value implements driver.Valuer
func (p PluginName) Value() (driver.Value, error) {
	return p.value, nil
}

// Scan implements sql.Scanner
func (p *PluginName) Scan(value interface{}) error {
	if value == nil {
		*p = PluginName{}
		return nil
	}

	switch v := value.(type) {
	case string:
		name, err := NewPluginName(v)
		if err != nil {
			return err
		}
		*p = name
		return nil
	case []byte:
		name, err := NewPluginName(string(v))
		if err != nil {
			return err
		}
		*p = name
		return nil
	default:
		return fmt.Errorf("cannot scan %T into PluginName", value)
	}
}
