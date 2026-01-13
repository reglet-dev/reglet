package sensitivedata_test

import (
	"testing"

	"github.com/reglet-dev/reglet/internal/infrastructure/sensitivedata"
	"github.com/stretchr/testify/assert"
)

func TestSortAndDeduplicateMatches_Empty(t *testing.T) {
	result := sensitivedata.SortAndDeduplicateMatches(nil)
	assert.Nil(t, result)

	result = sensitivedata.SortAndDeduplicateMatches([]sensitivedata.Match{})
	assert.Empty(t, result)
}

func TestSortAndDeduplicateMatches_Single(t *testing.T) {
	matches := []sensitivedata.Match{
		{Start: 5, End: 10, Secret: "hello"},
	}
	result := sensitivedata.SortAndDeduplicateMatches(matches)
	assert.Equal(t, matches, result)
}

func TestSortAndDeduplicateMatches_NoOverlap(t *testing.T) {
	matches := []sensitivedata.Match{
		{Start: 0, End: 5, Secret: "hello"},
		{Start: 10, End: 15, Secret: "world"},
	}
	result := sensitivedata.SortAndDeduplicateMatches(matches)
	assert.Equal(t, matches, result)
}

func TestSortAndDeduplicateMatches_UnsortedNoOverlap(t *testing.T) {
	matches := []sensitivedata.Match{
		{Start: 10, End: 15, Secret: "world"},
		{Start: 0, End: 5, Secret: "hello"},
	}
	result := sensitivedata.SortAndDeduplicateMatches(matches)

	expected := []sensitivedata.Match{
		{Start: 0, End: 5, Secret: "hello"},
		{Start: 10, End: 15, Secret: "world"},
	}
	assert.Equal(t, expected, result)
}

func TestSortAndDeduplicateMatches_OverlappingKeepsLonger(t *testing.T) {
	// "secret_key" overlaps with "secret"
	matches := []sensitivedata.Match{
		{Start: 0, End: 6, Secret: "secret"},      // shorter
		{Start: 0, End: 10, Secret: "secret_key"}, // longer, same start
	}
	result := sensitivedata.SortAndDeduplicateMatches(matches)

	// Should keep the longer match only
	expected := []sensitivedata.Match{
		{Start: 0, End: 10, Secret: "secret_key"},
	}
	assert.Equal(t, expected, result)
}

func TestSortAndDeduplicateMatches_PartialOverlap(t *testing.T) {
	// Two matches where the second starts before the first ends
	matches := []sensitivedata.Match{
		{Start: 0, End: 10, Secret: "secret_key"},
		{Start: 5, End: 15, Secret: "key_value"}, // overlaps at position 5-10
	}
	result := sensitivedata.SortAndDeduplicateMatches(matches)

	// First match wins because it comes first
	expected := []sensitivedata.Match{
		{Start: 0, End: 10, Secret: "secret_key"},
	}
	assert.Equal(t, expected, result)
}

func TestSortAndDeduplicateMatches_AdjacentMatches(t *testing.T) {
	// Matches that are adjacent (no gap) should both be kept
	matches := []sensitivedata.Match{
		{Start: 0, End: 5, Secret: "hello"},
		{Start: 5, End: 10, Secret: "world"},
	}
	result := sensitivedata.SortAndDeduplicateMatches(matches)
	assert.Equal(t, matches, result)
}

func TestSortAndDeduplicateMatches_NestedMatch(t *testing.T) {
	// Nested: "secret" is fully contained within "my_secret_key"
	matches := []sensitivedata.Match{
		{Start: 0, End: 13, Secret: "my_secret_key"},
		{Start: 3, End: 9, Secret: "secret"},
	}
	result := sensitivedata.SortAndDeduplicateMatches(matches)

	// Outer match wins
	expected := []sensitivedata.Match{
		{Start: 0, End: 13, Secret: "my_secret_key"},
	}
	assert.Equal(t, expected, result)
}

func TestApplyReplacements_Empty(t *testing.T) {
	result := sensitivedata.ApplyReplacements("hello world", nil, func(s string) string {
		return "[REDACTED]"
	})
	assert.Equal(t, "hello world", result)
}

func TestApplyReplacements_Single(t *testing.T) {
	matches := []sensitivedata.Match{
		{Start: 0, End: 5, Secret: "hello"},
	}
	result := sensitivedata.ApplyReplacements("hello world", matches, func(s string) string {
		return "[REDACTED]"
	})
	assert.Equal(t, "[REDACTED] world", result)
}

func TestApplyReplacements_Multiple(t *testing.T) {
	matches := []sensitivedata.Match{
		{Start: 0, End: 5, Secret: "hello"},
		{Start: 6, End: 11, Secret: "world"},
	}
	result := sensitivedata.ApplyReplacements("hello world", matches, func(s string) string {
		return "[X]"
	})
	assert.Equal(t, "[X] [X]", result)
}

func TestApplyReplacements_PreservesNonMatchedText(t *testing.T) {
	input := "prefix secret_value suffix"
	matches := []sensitivedata.Match{
		{Start: 7, End: 19, Secret: "secret_value"},
	}
	result := sensitivedata.ApplyReplacements(input, matches, func(s string) string {
		return "[REDACTED]"
	})
	assert.Equal(t, "prefix [REDACTED] suffix", result)
}

func TestApplyReplacements_DifferentReplacementLengths(t *testing.T) {
	// Short secret replaced with long replacement
	input := "abc XYZ def"
	matches := []sensitivedata.Match{
		{Start: 4, End: 7, Secret: "XYZ"},
	}
	result := sensitivedata.ApplyReplacements(input, matches, func(s string) string {
		return "[VERY_LONG_REDACTED_STRING]"
	})
	assert.Equal(t, "abc [VERY_LONG_REDACTED_STRING] def", result)
}
