// Package values contains domain value objects that encapsulate
// primitive types with validation and such.
package values

import (
	"fmt"

	"github.com/google/uuid"
)

// ExecutionID uniquely identifies a profile execution.
// This is critical for persistence, distributed execution, and result tracking.
type ExecutionID struct {
	value uuid.UUID
}

// NewExecutionID creates a new random execution ID
func NewExecutionID() ExecutionID {
	return ExecutionID{value: uuid.New()}
}

// ParseExecutionID parses a string into an ExecutionID
func ParseExecutionID(s string) (ExecutionID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return ExecutionID{}, fmt.Errorf("invalid execution ID: %w", err)
	}
	return ExecutionID{value: id}, nil
}

// MustParseExecutionID parses a string or panics (for tests only)
func MustParseExecutionID(s string) ExecutionID {
	id, err := ParseExecutionID(s)
	if err != nil {
		panic(err)
	}
	return id
}

// FromUUID creates an ExecutionID from a uuid.UUID
func FromUUID(id uuid.UUID) ExecutionID {
	return ExecutionID{value: id}
}

// String returns the string representation
func (e ExecutionID) String() string {
	return e.value.String()
}

// UUID returns the underlying uuid.UUID
func (e ExecutionID) UUID() uuid.UUID {
	return e.value
}

// IsZero returns true if this is the zero value
func (e ExecutionID) IsZero() bool {
	return e.value == uuid.Nil
}

// Equals checks if two ExecutionIDs are equal
func (e ExecutionID) Equals(other ExecutionID) bool {
	return e.value == other.value
}

// MarshalJSON implements json.Marshaler
func (e ExecutionID) MarshalJSON() ([]byte, error) {
	return []byte(`"` + e.value.String() + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler
func (e *ExecutionID) UnmarshalJSON(data []byte) error {
	// Remove quotes
	s := string(data)
	if len(s) < 2 {
		return fmt.Errorf("invalid execution ID JSON")
	}
	s = s[1 : len(s)-1]

	id, err := ParseExecutionID(s)
	if err != nil {
		return err
	}
	*e = id
	return nil
}
