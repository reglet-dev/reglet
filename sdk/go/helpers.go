package sdk

import (
	"fmt"
)

// GetString safely extracts a string value from Config.
// Returns the value and true if found and is a string, otherwise returns empty string and false.
func GetString(config Config, key string) (string, bool) {
	v, ok := config[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// GetInt safely extracts an int value from Config.
// Returns the value and true if found and is a numeric type, otherwise returns 0 and false.
// Handles both int and float64 (JSON numbers are often decoded as float64).
func GetInt(config Config, key string) (int, bool) {
	v, ok := config[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

// GetFloat safely extracts a float64 value from Config.
// Returns the value and true if found and is a numeric type, otherwise returns 0 and false.
func GetFloat(config Config, key string) (float64, bool) {
	v, ok := config[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

// GetBool safely extracts a bool value from Config.
// Returns the value and true if found and is a bool, otherwise returns false and false.
func GetBool(config Config, key string) (bool, bool) {
	v, ok := config[key]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

// GetStringSlice safely extracts a []string value from Config.
// Returns the value and true if found and is a slice of strings, otherwise returns nil and false.
func GetStringSlice(config Config, key string) ([]string, bool) {
	v, ok := config[key]
	if !ok {
		return nil, false
	}
	// JSON arrays are decoded as []interface{}
	arr, ok := v.([]interface{})
	if !ok {
		return nil, false
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		s, ok := item.(string)
		if !ok {
			return nil, false
		}
		result = append(result, s)
	}
	return result, true
}

// MustGetString extracts a string value from Config or returns an error.
// Use this when the field is required.
func MustGetString(config Config, key string) (string, error) {
	s, ok := GetString(config, key)
	if !ok {
		return "", &ConfigError{
			Field: key,
			Err:   fmt.Errorf("required string field '%s' is missing or not a string", key),
		}
	}
	return s, nil
}

// MustGetInt extracts an int value from Config or returns an error.
// Use this when the field is required.
func MustGetInt(config Config, key string) (int, error) {
	i, ok := GetInt(config, key)
	if !ok {
		return 0, &ConfigError{
			Field: key,
			Err:   fmt.Errorf("required int field '%s' is missing or not a number", key),
		}
	}
	return i, nil
}

// MustGetBool extracts a bool value from Config or returns an error.
// Use this when the field is required.
func MustGetBool(config Config, key string) (bool, error) {
	b, ok := GetBool(config, key)
	if !ok {
		return false, &ConfigError{
			Field: key,
			Err:   fmt.Errorf("required bool field '%s' is missing or not a boolean", key),
		}
	}
	return b, nil
}

// GetStringDefault extracts a string value from Config with a default.
// Returns the value if found and is a string, otherwise returns the default.
func GetStringDefault(config Config, key, defaultValue string) string {
	s, ok := GetString(config, key)
	if !ok {
		return defaultValue
	}
	return s
}

// GetIntDefault extracts an int value from Config with a default.
// Returns the value if found and is numeric, otherwise returns the default.
func GetIntDefault(config Config, key string, defaultValue int) int {
	i, ok := GetInt(config, key)
	if !ok {
		return defaultValue
	}
	return i
}

// GetBoolDefault extracts a bool value from Config with a default.
// Returns the value if found and is a bool, otherwise returns the default.
func GetBoolDefault(config Config, key string, defaultValue bool) bool {
	b, ok := GetBool(config, key)
	if !ok {
		return defaultValue
	}
	return b
}
