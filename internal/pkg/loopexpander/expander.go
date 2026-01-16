// Package loopexpander provides template substitution utilities for loop observations.
// This is a pure utility package with no domain or infrastructure dependencies.
package loopexpander

import (
	"fmt"
	"regexp"
	"strings"
)

// loopVarPattern matches {{ .loop.X }} patterns.
var loopVarPattern = regexp.MustCompile(`\{\{\s*\.loop\.([a-zA-Z0-9_]+)\s*\}\}`)

// Context provides loop iteration context for template substitution.
type Context struct {
	Item   interface{}
	Index  int
	First  bool
	Last   bool
	Length int
}

// ResolveLoopItems extracts the list of items from a loop.items template expression.
// It expects the expression to be in the form "{{ .vars.xxx }}" and looks up the value.
func ResolveLoopItems(itemsExpr string, vars map[string]interface{}) ([]interface{}, error) {
	// Extract the variable path from the expression
	// Pattern: {{ .vars.xxx }}
	pattern := regexp.MustCompile(`\{\{\s*\.vars\.([a-zA-Z0-9_.]+)\s*\}\}`)
	matches := pattern.FindStringSubmatch(itemsExpr)
	if len(matches) < 2 {
		return nil, fmt.Errorf("invalid loop items expression: %s (expected {{ .vars.xxx }})", itemsExpr)
	}

	varPath := matches[1]

	// Look up the variable value
	value, err := LookupNestedVar(vars, varPath)
	if err != nil {
		return nil, err
	}

	// Convert to slice
	switch v := value.(type) {
	case []interface{}:
		return v, nil
	case []string:
		result := make([]interface{}, len(v))
		for i, s := range v {
			result[i] = s
		}
		return result, nil
	case []map[string]interface{}:
		result := make([]interface{}, len(v))
		for i, m := range v {
			result[i] = m
		}
		return result, nil
	default:
		return nil, fmt.Errorf("loop items must be a list, got %T", value)
	}
}

// LookupNestedVar looks up a variable value by path (e.g., "config.path").
func LookupNestedVar(vars map[string]interface{}, path string) (interface{}, error) {
	parts := strings.Split(path, ".")
	current := interface{}(vars)

	for i, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("variable path %s: cannot access %s (not a map)", path, strings.Join(parts[:i+1], "."))
		}

		value, exists := m[part]
		if !exists {
			return nil, fmt.Errorf("variable not found: %s", path)
		}

		current = value
	}

	return current, nil
}

// SubstituteLoopInMap recursively substitutes loop variables in map values.
func SubstituteLoopInMap(m map[string]interface{}, loopCtx *Context, customName string) map[string]interface{} {
	result := make(map[string]interface{})
	for key, value := range m {
		switch v := value.(type) {
		case string:
			result[key] = SubstituteLoopInString(v, loopCtx, customName)
		case map[string]interface{}:
			result[key] = SubstituteLoopInMap(v, loopCtx, customName)
		case []interface{}:
			newSlice := make([]interface{}, len(v))
			for i, elem := range v {
				if str, ok := elem.(string); ok {
					newSlice[i] = SubstituteLoopInString(str, loopCtx, customName)
				} else if nested, ok := elem.(map[string]interface{}); ok {
					newSlice[i] = SubstituteLoopInMap(nested, loopCtx, customName)
				} else {
					newSlice[i] = elem
				}
			}
			result[key] = newSlice
		default:
			result[key] = value
		}
	}
	return result
}

// SubstituteLoopInString replaces loop variable patterns in a string.
func SubstituteLoopInString(str string, loopCtx *Context, customName string) string {
	// Replace {{ .loop.X }} patterns
	result := loopVarPattern.ReplaceAllStringFunc(str, func(match string) string {
		submatches := loopVarPattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		key := submatches[1]
		switch key {
		case "item":
			return formatValue(loopCtx.Item)
		case "index":
			return fmt.Sprintf("%d", loopCtx.Index)
		case "first":
			return fmt.Sprintf("%t", loopCtx.First)
		case "last":
			return fmt.Sprintf("%t", loopCtx.Last)
		case "length":
			return fmt.Sprintf("%d", loopCtx.Length)
		default:
			// Try to access nested field in item
			if m, ok := loopCtx.Item.(map[string]interface{}); ok {
				if val, exists := m[key]; exists {
					return formatValue(val)
				}
			}
			return match
		}
	})

	// If custom name specified, also replace {{ .customName }} and {{ .customName.X }}
	if customName != "" {
		customPattern := regexp.MustCompile(`\{\{\s*\.` + regexp.QuoteMeta(customName) + `(?:\.([a-zA-Z0-9_]+))?\s*\}\}`)
		result = customPattern.ReplaceAllStringFunc(result, func(match string) string {
			submatches := customPattern.FindStringSubmatch(match)
			if len(submatches) < 1 {
				return match
			}
			if len(submatches) < 2 || submatches[1] == "" {
				// {{ .customName }} - return whole item
				return formatValue(loopCtx.Item)
			}
			// {{ .customName.field }} - access field
			if m, ok := loopCtx.Item.(map[string]interface{}); ok {
				if val, exists := m[submatches[1]]; exists {
					return formatValue(val)
				}
			}
			return match
		})
	}

	return result
}

// formatValue converts a value to its string representation.
func formatValue(v interface{}) string {
	return fmt.Sprintf("%v", v)
}
