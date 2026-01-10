package sdk

import (
	"fmt"
)

// GetString extracts a string from config, returning (value, found).
func GetString(config Config, key string) (string, bool) {
	v, ok := config[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// GetInt extracts an int from config, handling int, int64, and float64.
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

// GetFloat extracts a float64 from config, handling float64, int, and int64.
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

// GetBool extracts a bool from config, returning (value, found).
func GetBool(config Config, key string) (bool, bool) {
	v, ok := config[key]
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

// GetStringSlice extracts a []string from config, returning (value, found).
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

// MustGetString extracts a required string from config or returns error.
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

// MustGetInt extracts a required int from config or returns error.
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

// MustGetBool extracts a required bool from config or returns error.
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

// GetStringDefault extracts a string from config or returns the default value.
func GetStringDefault(config Config, key, defaultValue string) string {
	s, ok := GetString(config, key)
	if !ok {
		return defaultValue
	}
	return s
}

// GetIntDefault extracts an int from config or returns the default value.
func GetIntDefault(config Config, key string, defaultValue int) int {
	i, ok := GetInt(config, key)
	if !ok {
		return defaultValue
	}
	return i
}

// GetBoolDefault extracts a bool from config or returns the default value.
func GetBoolDefault(config Config, key string, defaultValue bool) bool {
	b, ok := GetBool(config, key)
	if !ok {
		return defaultValue
	}
	return b
}
