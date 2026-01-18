// Package config provides configuration parsing and variable handling.
package config

import (
	"fmt"
	"os"
	"strings"
)

// ReadValueFromEnv reads a value from an environment variable for --set-env.
// Returns error if the environment variable is not set.
func ReadValueFromEnv(envVar string) (string, error) {
	value, exists := os.LookupEnv(envVar)
	if !exists {
		return "", fmt.Errorf("environment variable not set: %s", envVar)
	}
	return value, nil
}

// ParseSetEnv parses a --set-env argument (key=ENV_VAR) and returns the key and value.
func ParseSetEnv(input string) (string, interface{}, error) {
	parts := strings.SplitN(input, "=", 2)
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("invalid --set-env format: expected key=ENV_VAR, got %q", input)
	}

	key := parts[0]
	envVar := parts[1]

	// Validate key format
	if !ValidKeyPattern.MatchString(key) {
		return "", nil, fmt.Errorf("invalid key %q: must match pattern [a-zA-Z_][a-zA-Z0-9_.]*", key)
	}

	// Read value from environment
	value, err := ReadValueFromEnv(envVar)
	if err != nil {
		return "", nil, err
	}

	return key, value, nil
}
