// Package sensitivedata provides tools for managing and protecting sensitive information
// such as secrets, passwords, and tokens.
package sensitivedata

import (
	"sort"
	"sync"

	ahocorasick "github.com/BobuSumisu/aho-corasick"
	"github.com/reglet-dev/reglet/internal/application/ports"
)

// SensitiveStringMatcher finds and replaces multiple sensitive values in a string.
// Implementations should be safe for concurrent use.
type SensitiveStringMatcher interface {
	// ReplaceAll finds all occurrences of sensitive values in input and replaces them.
	// The replacement function is called for each match to get the replacement string.
	ReplaceAll(input string, replacement func(secret string) string) string
}

// ahocorasickMatcher implements SensitiveStringMatcher using the Aho-Corasick algorithm.
// This allows O(n) multi-pattern matching instead of O(n * patterns).
//
// The trie is lazily rebuilt when the number of tracked secrets changes.
type ahocorasickMatcher struct {
	provider  ports.SensitiveValueProvider
	trie      *ahocorasick.Trie
	lastCount int
	mu        sync.RWMutex
}

// NewAhoCorasickMatcher creates a new matcher backed by a SensitiveValueProvider.
func NewAhoCorasickMatcher(provider ports.SensitiveValueProvider) SensitiveStringMatcher {
	return &ahocorasickMatcher{
		provider: provider,
	}
}

// ReplaceAll finds all occurrences of sensitive values and replaces them in a single pass.
//
// Algorithm:
// 1. Check if trie needs rebuilding (secret count changed)
// 2. Find all matches using Aho-Corasick (O(n + m) where n=input length, m=matches)
// 3. Sort matches by position and handle overlaps
// 4. Build result by replacing matched regions
func (m *ahocorasickMatcher) ReplaceAll(input string, replacement func(secret string) string) string {
	if input == "" {
		return ""
	}

	trie := m.getTrie()
	if trie == nil {
		return input // No secrets to match
	}

	matches := trie.MatchString(input)
	if len(matches) == 0 {
		return input
	}

	// Sort matches by position for proper processing
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Pos() < matches[j].Pos()
	})

	// Build result, handling overlapping matches
	var result []byte
	lastEnd := int64(0)

	for _, match := range matches {
		pos := match.Pos()
		secret := match.MatchString()
		end := pos + int64(len(secret))

		// Skip if this match overlaps with a previous replacement
		if pos < lastEnd {
			continue
		}

		// Append text before this match
		if pos > lastEnd {
			result = append(result, input[lastEnd:pos]...)
		}

		// Append replacement
		result = append(result, replacement(secret)...)
		lastEnd = end
	}

	// Append remaining text after last match
	if lastEnd < int64(len(input)) {
		result = append(result, input[lastEnd:]...)
	}

	return string(result)
}

// getTrie returns the current Aho-Corasick trie, rebuilding if necessary.
func (m *ahocorasickMatcher) getTrie() *ahocorasick.Trie {
	// Fast path: read lock to check if rebuild is needed
	m.mu.RLock()
	secrets := m.provider.AllValues()
	currentCount := len(secrets)

	if m.trie != nil && currentCount == m.lastCount {
		trie := m.trie
		m.mu.RUnlock()
		return trie
	}
	m.mu.RUnlock()

	// Slow path: write lock to rebuild
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	secrets = m.provider.AllValues()
	currentCount = len(secrets)

	if m.trie != nil && currentCount == m.lastCount {
		return m.trie
	}

	// Rebuild trie
	if currentCount == 0 {
		m.trie = nil
		m.lastCount = 0
		return nil
	}

	// Filter empty strings
	filtered := make([]string, 0, len(secrets))
	for _, s := range secrets {
		if s != "" {
			filtered = append(filtered, s)
		}
	}

	if len(filtered) == 0 {
		m.trie = nil
		m.lastCount = 0
		return nil
	}

	m.trie = ahocorasick.NewTrieBuilder().AddStrings(filtered).Build()
	m.lastCount = currentCount
	return m.trie
}
