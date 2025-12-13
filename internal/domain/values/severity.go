package values

import (
	"database/sql/driver"
	"fmt"
	"strings"
)

// Severity represents the severity level of a control.
// Enforces valid severity values and provides ordering.
type Severity struct {
	value SeverityLevel
}

// SeverityLevel is the internal representation
type SeverityLevel int

const (
	SeverityUnknown  SeverityLevel = 0
	SeverityLow      SeverityLevel = 1
	SeverityMedium   SeverityLevel = 2
	SeverityHigh     SeverityLevel = 3
	SeverityCritical SeverityLevel = 4
)

// Predefined severity values
var (
	SevUnknown  = Severity{SeverityUnknown}
	SevLow      = Severity{SeverityLow}
	SevMedium   = Severity{SeverityMedium}
	SevHigh     = Severity{SeverityHigh}
	SevCritical = Severity{SeverityCritical}
)

// NewSeverity creates a Severity from string
func NewSeverity(s string) (Severity, error) {
	s = strings.ToLower(strings.TrimSpace(s))

	switch s {
	case "low":
		return SevLow, nil
	case "medium":
		return SevMedium, nil
	case "high":
		return SevHigh, nil
	case "critical":
		return SevCritical, nil
	case "":
		return SevUnknown, nil
	default:
		return Severity{}, fmt.Errorf("invalid severity: %s", s)
	}
}

// MustNewSeverity creates a Severity or panics
func MustNewSeverity(s string) Severity {
	sev, err := NewSeverity(s)
	if err != nil {
		panic(err)
	}
	return sev
}

// String returns the string representation
func (s Severity) String() string {
	switch s.value {
	case SeverityLow:
		return "low"
	case SeverityMedium:
		return "medium"
	case SeverityHigh:
		return "high"
	case SeverityCritical:
		return "critical"
	default:
		return ""
	}
}

// Level returns the numeric severity level (for ordering)
func (s Severity) Level() int {
	return int(s.value)
}

// IsHigherThan returns true if this severity is higher than the other
func (s Severity) IsHigherThan(other Severity) bool {
	return s.value > other.value
}

// IsHigherOrEqual returns true if this severity is higher or equal to the other
func (s Severity) IsHigherOrEqual(other Severity) bool {
	return s.value >= other.value
}

// Equals checks if two severities are equal
func (s Severity) Equals(other Severity) bool {
	return s.value == other.value
}

// MarshalJSON implements json.Marshaler
func (s Severity) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.String() + `"`), nil
}

// UnmarshalJSON implements json.Unmarshaler
func (s *Severity) UnmarshalJSON(data []byte) error {
	str := string(data)
	if len(str) < 2 {
		return fmt.Errorf("invalid severity JSON")
	}
	str = str[1 : len(str)-1]

	sev, err := NewSeverity(str)
	if err != nil {
		return err
	}
	*s = sev
	return nil
}

// Value implements driver.Valuer
func (s Severity) Value() (driver.Value, error) {
	return s.String(), nil
}

// Scan implements sql.Scanner
func (s *Severity) Scan(value interface{}) error {
	if value == nil {
		*s = SevUnknown
		return nil
	}

	switch v := value.(type) {
	case string:
		sev, err := NewSeverity(v)
		if err != nil {
			return err
		}
		*s = sev
		return nil
	case []byte:
		sev, err := NewSeverity(string(v))
		if err != nil {
			return err
		}
		*s = sev
		return nil
	default:
		return fmt.Errorf("cannot scan %T into Severity", value)
	}
}
