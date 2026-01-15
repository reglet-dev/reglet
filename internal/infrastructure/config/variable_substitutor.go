package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/spf13/cast"
)

// Variable pattern: {{ .vars.key }}
var varPattern = regexp.MustCompile(`\{\{\s*\.vars\.([a-zA-Z0-9_.]+)\s*\}\}`)

// Secret pattern: {{ secret "key" }}
var secretPattern = regexp.MustCompile(`\{\{\s*secret\s+"([a-zA-Z0-9_.-]+)"\s*\}\}`)

// VariableSubstitutor performs variable substitution in profiles.
type VariableSubstitutor struct {
	resolver ports.SecretResolver
}

// NewVariableSubstitutor creates a new variable substitutor.
func NewVariableSubstitutor(resolver ports.SecretResolver) *VariableSubstitutor {
	return &VariableSubstitutor{
		resolver: resolver,
	}
}

// Substitute performs simple variable substitution in a profile.
// It replaces {{ .vars.key }} patterns with values from the profile's vars map.
// Supports nested paths like {{ .vars.paths.config }}.
// Returns an error if a referenced variable is not found.
// Modifies the profile in place.
func (s *VariableSubstitutor) Substitute(profile *entities.Profile) error {
	// Always substitute in controls, even if vars is empty, because we might have secrets
	// However, the original logic checked for empty vars.
	// For secrets, we don't need profile.Vars.
	// So we should remove the 'if len(profile.Vars) == 0' check if we want secrets to work without vars.
	// But let's check if existing logic relied on it.

	// Substitute variables in each control
	for i := range profile.Controls.Items {
		ctrl := &profile.Controls.Items[i]

		// Substitute in control fields
		var err error
		ctrl.Description, err = s.substituteInString(ctrl.Description, profile.Vars)
		if err != nil {
			return fmt.Errorf("control %s: %w", ctrl.ID, err)
		}

		// Substitute in each observation config
		for j := range ctrl.ObservationDefinitions {
			obs := &ctrl.ObservationDefinitions[j]

			if err := s.substituteInMap(obs.Config, profile.Vars); err != nil {
				return fmt.Errorf("control %s, observation %d: %w", ctrl.ID, j, err)
			}

			// Substitute in expect expressions
			for k := range obs.Expect {
				obs.Expect[k], err = s.substituteInString(obs.Expect[k], profile.Vars)
				if err != nil {
					return fmt.Errorf("control %s, observation %d, expect %d: %w", ctrl.ID, j, k, err)
				}
			}
		}
	}

	return nil
}

// substituteInString replaces patterns with values.
func (s *VariableSubstitutor) substituteInString(str string, vars map[string]interface{}) (string, error) {
	var lastErr error

	// 1. Substitute variables: {{ .vars.key }}
	result := varPattern.ReplaceAllStringFunc(str, func(match string) string {
		// Extract the variable path (e.g., "config.path" from "{{ .vars.config.path }}")
		submatches := varPattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			lastErr = fmt.Errorf("invalid variable pattern: %s", match)
			return match
		}

		varPath := submatches[1]

		// Look up the variable value
		value, err := lookupVar(vars, varPath)
		if err != nil {
			lastErr = err
			return match
		}

		// Convert value to string
		return fmt.Sprintf("%v", value)
	})

	if lastErr != nil {
		return "", lastErr
	}

	// 2. Substitute secrets: {{ secret "key" }}
	// Only if a resolver is configured (to allow testing/usage without secrets)
	if s.resolver != nil {
		result = secretPattern.ReplaceAllStringFunc(result, func(match string) string {
			submatches := secretPattern.FindStringSubmatch(match)
			if len(submatches) < 2 {
				lastErr = fmt.Errorf("invalid secret pattern: %s", match)
				return match
			}

			secretName := submatches[1]
			value, err := s.resolver.Resolve(secretName)
			if err != nil {
				lastErr = fmt.Errorf("resolving secret %s: %w", secretName, err)
				return match
			}

			return value
		})
	}

	if lastErr != nil {
		return "", lastErr
	}

	return result, nil
}

// substituteInMap recursively substitutes variables in map values.
// Modifies the map in place.
func (s *VariableSubstitutor) substituteInMap(m map[string]interface{}, vars map[string]interface{}) error {
	for key, value := range m {
		switch v := value.(type) {
		case string:
			// Substitute variables in string value
			substituted, err := s.substituteInString(v, vars)
			if err != nil {
				return fmt.Errorf("key %s: %w", key, err)
			}
			m[key] = substituted

		case map[string]interface{}:
			// Recursively substitute in nested map
			if err := s.substituteInMap(v, vars); err != nil {
				return fmt.Errorf("key %s: %w", key, err)
			}

		case []interface{}:
			// Substitute in array elements
			for i, elem := range v {
				if str, ok := elem.(string); ok {
					substituted, err := s.substituteInString(str, vars)
					if err != nil {
						return fmt.Errorf("key %s[%d]: %w", key, i, err)
					}
					v[i] = substituted
				} else if nested, ok := elem.(map[string]interface{}); ok {
					if err := s.substituteInMap(nested, vars); err != nil {
						return fmt.Errorf("key %s[%d]: %w", key, i, err)
					}
				}
			}

			// Other types (int, bool, etc.) don't need substitution
		}
	}

	return nil
}

// lookupVar looks up a variable value by path (e.g., "config.path").
// Supports nested paths using dot notation.
// Handles all numeric types robustly using spf13/cast.
func lookupVar(vars map[string]interface{}, path string) (interface{}, error) {
	parts := strings.Split(path, ".")
	current := interface{}(vars)

	for i, part := range parts {
		// Try to access current as a map
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("variable path %s: cannot access %s (not a map)", path, strings.Join(parts[:i+1], "."))
		}

		// Look up the next part
		value, exists := m[part]
		if !exists {
			return nil, fmt.Errorf("variable not found: %s", path)
		}

		current = value
	}

	// Normalize numeric types using cast library for robust handling.
	// This handles int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64,
	// float32, float64, and other edge cases automatically.
	switch v := current.(type) {
	case string:
		return v, nil
	case bool:
		return v, nil
	case map[string]interface{}:
		// For nested maps, allow them (might be used in complex configs)
		return v, nil
	case []interface{}:
		// Arrays are allowed
		return v, nil
	case int, int8, int16, int32, int64:
		// Normalize all signed integers to int64
		return cast.ToInt64(v), nil
	case uint, uint8, uint16, uint32, uint64:
		// Normalize all unsigned integers to uint64
		return cast.ToUint64(v), nil
	case float32, float64:
		// Normalize all floats to float64
		return cast.ToFloat64(v), nil
	default:
		// For any other type, try cast library as fallback
		// This handles custom numeric types, etc.
		if num, err := cast.ToInt64E(v); err == nil {
			return num, nil
		}
		if num, err := cast.ToFloat64E(v); err == nil {
			return num, nil
		}
		// Return as-is if we can't convert
		return v, nil
	}
}
