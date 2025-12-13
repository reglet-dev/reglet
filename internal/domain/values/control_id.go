package values

import (
	"database/sql/driver"
	"fmt"
	"strings"
)

// ControlID uniquely identifies a control within a profile.
// Enforces non-empty, trimmed identifiers.
type ControlID struct {
	value string
}

// NewControlID creates a new ControlID with validation
func NewControlID(id string) (ControlID, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return ControlID{}, fmt.Errorf("control ID cannot be empty")
	}
	// Optional: Add more validation (no special chars, length limits, etc.)
	return ControlID{value: id}, nil
}

// MustNewControlID creates a ControlID or panics (for tests/constants)
func MustNewControlID(id string) ControlID {
	cid, err := NewControlID(id)
	if err != nil {
		panic(err)
	}
	return cid
}

// String returns the string representation
func (c ControlID) String() string {
	return c.value
}

// IsEmpty returns true if this is the zero value
func (c ControlID) IsEmpty() bool {
	return c.value == ""
}

// Equals checks if two ControlIDs are equal
func (c ControlID) Equals(other ControlID) bool {
	return c.value == other.value
}

// MarshalJSON implements json.Marshaler
func (c ControlID) MarshalJSON() ([]byte, error) {
	return []byte(`"` + c.value + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler
func (c *ControlID) UnmarshalJSON(data []byte) error {
	// Remove quotes
	s := string(data)
	if len(s) < 2 {
		return fmt.Errorf("invalid control ID JSON")
	}
	s = s[1 : len(s)-1]

	id, err := NewControlID(s)
	if err != nil {
		return err
	}
	*c = id
	return nil
}

// Value implements driver.Valuer for database/sql
func (c ControlID) Value() (driver.Value, error) {
	return c.value, nil
}

// Scan implements sql.Scanner for database/sql
func (c *ControlID) Scan(value interface{}) error {
	if value == nil {
		*c = ControlID{}
		return nil
	}

	switch v := value.(type) {
	case string:
		id, err := NewControlID(v)
		if err != nil {
			return err
		}
		*c = id
		return nil
	case []byte:
		id, err := NewControlID(string(v))
		if err != nil {
			return err
		}
		*c = id
		return nil
	default:
		return fmt.Errorf("cannot scan %T into ControlID", value)
	}
}
