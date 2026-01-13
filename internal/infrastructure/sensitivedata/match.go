// Package sensitivedata provides tools for managing and protecting sensitive information
// such as secrets, passwords, and tokens.
package sensitivedata

import "sort"

// Match represents a sensitive value found at a specific position in an input string.
// Used for collecting matches from multiple detectors before applying replacements.
type Match struct {
	Secret string // The matched secret value
	Start  int    // Start position (inclusive)
	End    int    // End position (exclusive)
}

// SortAndDeduplicateMatches sorts matches by position and resolves overlaps.
//
// Overlap resolution strategy:
//   - Matches are sorted by start position
//   - For overlapping matches, the longer (more specific) match wins
//   - For equal-length overlaps, the earlier match wins
//
// This ensures deterministic behavior and prefers more specific detections.
func SortAndDeduplicateMatches(matches []Match) []Match {
	if len(matches) <= 1 {
		return matches
	}

	// Sort by start position, then by length descending (longer first)
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Start != matches[j].Start {
			return matches[i].Start < matches[j].Start
		}
		// Same start: prefer longer match
		return len(matches[i].Secret) > len(matches[j].Secret)
	})

	// Remove overlapping matches, keeping the first (longest at each position)
	result := make([]Match, 0, len(matches))
	lastEnd := 0

	for _, m := range matches {
		if m.Start >= lastEnd {
			result = append(result, m)
			lastEnd = m.End
		}
		// Skip matches that start before the last match ended
	}

	return result
}

// ApplyReplacements applies all match replacements to the input string.
// Matches must be non-overlapping and sorted by position.
//
// The replacement function is called for each match to determine the replacement text.
// Replacements are applied from end-to-start to preserve string positions.
func ApplyReplacements(input string, matches []Match, replacement func(secret string) string) string {
	if len(matches) == 0 {
		return input
	}

	// Work with bytes for efficient slicing
	result := []byte(input)

	// Apply replacements in reverse order to preserve positions
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		repl := replacement(m.Secret)

		// Replace [Start:End] with replacement
		newResult := make([]byte, 0, len(result)-len(m.Secret)+len(repl))
		newResult = append(newResult, result[:m.Start]...)
		newResult = append(newResult, repl...)
		newResult = append(newResult, result[m.End:]...)
		result = newResult
	}

	return string(result)
}
