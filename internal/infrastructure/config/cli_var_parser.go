// Package config provides configuration parsing and variable handling.
package config

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidKeyPattern defines valid variable key format.
// Keys must start with a letter or underscore, followed by letters, digits, underscores, or dots.
var ValidKeyPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_.]*$`)

// intPattern matches integers: 0 or non-zero integers without leading zeros.
var intPattern = regexp.MustCompile(`^-?[1-9][0-9]*$|^0$`)

// floatPattern matches floats: integer part followed by decimal point and fractional part.
var floatPattern = regexp.MustCompile(`^-?[0-9]+\.[0-9]+$`)

// ParseCLIVarResult contains the result of parsing a CLI variable string.
type ParseCLIVarResult struct {
	Key      string
	Value    interface{}
	RawValue string
}

// ParseCLIVar parses a key=value string from --set flag.
// Returns the parsed key and typed value. Type detection is conservative:
//   - Integers: 0, 42, -7, 1000 (no leading zeros except for 0 itself)
//   - Floats: 3.14, -0.5 (must have both integer and decimal parts)
//   - Booleans: only literal "true" and "false" (case-sensitive)
//   - Everything else: string (including 007, 1.0.0, etc.)
func ParseCLIVar(input string) (ParseCLIVarResult, error) {
	// Split on first = only
	parts := strings.SplitN(input, "=", 2)
	if len(parts) != 2 {
		return ParseCLIVarResult{}, fmt.Errorf("invalid format: expected key=value, got %q", input)
	}

	key := parts[0]
	rawValue := parts[1]

	// Validate key format
	if !ValidKeyPattern.MatchString(key) {
		return ParseCLIVarResult{}, fmt.Errorf("invalid key %q: must match pattern [a-zA-Z_][a-zA-Z0-9_.]*", key)
	}

	// Detect and convert value type
	value := DetectValueType(rawValue)

	return ParseCLIVarResult{
		Key:      key,
		Value:    value,
		RawValue: rawValue,
	}, nil
}

// DetectValueType attempts to detect the type of a string value.
// Uses conservative rules to avoid surprises:
//   - Integer: ^-?[1-9][0-9]*$ or ^0$
//   - Float: ^-?[0-9]+\.[0-9]+$
//   - Boolean: only literal "true" or "false"
//   - Everything else: string
func DetectValueType(s string) interface{} {
	// Check for boolean first (exact match)
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}

	// Check for integer pattern
	if intPattern.MatchString(s) {
		// Parse as int64
		var val int64
		if _, err := fmt.Sscanf(s, "%d", &val); err == nil {
			return val
		}
	}

	// Check for float pattern
	if floatPattern.MatchString(s) {
		// Parse as float64
		var val float64
		if _, err := fmt.Sscanf(s, "%f", &val); err == nil {
			return val
		}
	}

	// Default to string
	return s
}

// SetNestedValue sets a value at a dot-notated path in a nested map structure.
// Creates intermediate maps as needed. Overwrites existing values.
// Example: SetNestedValue(m, "a.b.c", 42) sets m["a"]["b"]["c"] = 42
func SetNestedValue(m map[string]interface{}, path string, value interface{}) error {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return fmt.Errorf("empty path")
	}

	current := m
	// Navigate/create intermediate maps
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		if part == "" {
			return fmt.Errorf("empty path segment at position %d", i)
		}

		next, exists := current[part]
		if !exists {
			// Create intermediate map
			newMap := make(map[string]interface{})
			current[part] = newMap
			current = newMap
		} else {
			// Check if existing value is a map
			nextMap, ok := next.(map[string]interface{})
			if !ok {
				return fmt.Errorf("cannot create nested path: %q is not a map at %q", strings.Join(parts[:i+1], "."), part)
			}
			current = nextMap
		}
	}

	// Set the final value
	finalKey := parts[len(parts)-1]
	if finalKey == "" {
		return fmt.Errorf("empty final path segment")
	}
	current[finalKey] = value

	return nil
}

// ParseMultipleCLIVars parses multiple key=value strings and returns a nested map.
// Later values for the same key override earlier ones ("last wins" semantics).
func ParseMultipleCLIVars(inputs []string) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for _, input := range inputs {
		parsed, err := ParseCLIVar(input)
		if err != nil {
			return nil, fmt.Errorf("parsing %q: %w", input, err)
		}

		if err := SetNestedValue(result, parsed.Key, parsed.Value); err != nil {
			return nil, fmt.Errorf("setting %q: %w", parsed.Key, err)
		}
	}

	return result, nil
}
