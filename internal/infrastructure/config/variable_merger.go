// Package config provides configuration parsing and variable handling.
package config

// MergeCLIVars merges CLI variables into profile variables.
// CLI variables override profile variables at the same path.
// This implements the merge semantics from the data model:
//   - Scalar override: CLI value replaces profile value at same path
//   - Nested creation: Missing intermediate maps are NOT created (use SetNestedValue if needed)
//   - Type coercion: CLI type wins regardless of profile type
//   - Security: CLI values are literal strings, never re-parsed as templates
func MergeCLIVars(profileVars, cliVars map[string]interface{}) map[string]interface{} {
	if profileVars == nil {
		profileVars = make(map[string]interface{})
	}
	if cliVars == nil {
		return profileVars
	}

	// Deep copy profile vars to avoid mutation
	result := deepCopyMap(profileVars)

	// Merge CLI vars (CLI wins)
	mergeInto(result, cliVars)

	return result
}

// mergeInto recursively merges src into dst, with src taking precedence.
func mergeInto(dst, src map[string]interface{}) {
	for key, srcVal := range src {
		dstVal, exists := dst[key]
		if !exists {
			// Key doesn't exist in dst, just set it
			dst[key] = deepCopy(srcVal)
			continue
		}

		// Both exist, check if both are maps for recursive merge
		srcMap, srcIsMap := srcVal.(map[string]interface{})
		dstMap, dstIsMap := dstVal.(map[string]interface{})

		if srcIsMap && dstIsMap {
			// Both are maps, merge recursively
			mergeInto(dstMap, srcMap)
		} else {
			// Otherwise, src wins (overwrite)
			dst[key] = deepCopy(srcVal)
		}
	}
}

// deepCopyMap creates a deep copy of a map.
func deepCopyMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = deepCopy(v)
	}
	return result
}

// deepCopy creates a deep copy of a value.
func deepCopy(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		return deepCopyMap(val)
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, elem := range val {
			result[i] = deepCopy(elem)
		}
		return result
	default:
		// Primitive types (string, int64, float64, bool, etc.) are copied by value
		return v
	}
}
