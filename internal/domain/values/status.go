package values

import (
	"database/sql/driver"
	"fmt"
)

// Status represents the status of a control or observation.
type Status string

const (
	// StatusPass indicates the check passed
	StatusPass Status = "pass"
	// StatusFail indicates the check failed (but ran successfully)
	StatusFail Status = "fail"
	// StatusError indicates the check encountered an error
	StatusError Status = "error"
	// StatusSkipped indicates the check was skipped (dependency failure or filtered)
	StatusSkipped Status = "skipped"
)

// Precedence returns the numeric precedence of this status.
// Higher values indicate higher priority in aggregation.
// Used by status aggregator to determine control status.
//
// Precedence: Fail (3) > Error (2) > Skipped (1) > Pass (0)
func (s Status) Precedence() int {
	switch s {
	case StatusFail:
		return 3
	case StatusError:
		return 2
	case StatusSkipped:
		return 1
	case StatusPass:
		return 0
	default:
		return -1
	}
}

// IsFailure returns true if this status represents a failure or error
func (s Status) IsFailure() bool {
	return s == StatusFail || s == StatusError
}

// IsSuccess returns true if this status represents success
func (s Status) IsSuccess() bool {
	return s == StatusPass
}

// IsSkipped returns true if this status represents a skip
func (s Status) IsSkipped() bool {
	return s == StatusSkipped
}

// Validate returns an error if the status value is invalid
func (s Status) Validate() error {
	switch s {
	case StatusPass, StatusFail, StatusError, StatusSkipped:
		return nil
	default:
		return fmt.Errorf("invalid status: %s", s)
	}
}

// Value implements driver.Valuer for database/sql
func (s Status) Value() (driver.Value, error) {
	return string(s), nil
}

// Scan implements sql.Scanner for database/sql
func (s *Status) Scan(value interface{}) error {
	if value == nil {
		*s = ""
		return nil
	}

	switch v := value.(type) {
	case string:
		status := Status(v)
		if err := status.Validate(); err != nil {
			return err
		}
		*s = status
		return nil
	case []byte:
		status := Status(v)
		if err := status.Validate(); err != nil {
			return err
		}
		*s = status
		return nil
	default:
		return fmt.Errorf("cannot scan %T into Status", value)
	}
}
