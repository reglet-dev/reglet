// Package sensitivedata provides tools for managing and protecting sensitive information
// such as secrets, passwords, and tokens.
package sensitivedata

import (
	"hash/crc64"
	"io"
	"sort"
	"sync"

	ahocorasick "github.com/BobuSumisu/aho-corasick"
	"github.com/reglet-dev/reglet/internal/application/ports"
)

// SensitiveStringMatcher finds and replaces multiple sensitive values in a string.
// Implementations should be safe for concurrent use.
type SensitiveStringMatcher interface {
	// FindAll returns all match positions without performing replacement.
	// Matches may overlap and are not sorted.
	FindAll(input string) []Match

	// ReplaceAll finds all occurrences of sensitive values in input and replaces them.
	// The replacement function is called for each match to get the replacement string.
	ReplaceAll(input string, replacement func(secret string) string) string
}

// ahocorasickMatcher implements SensitiveStringMatcher using the Aho-Corasick algorithm.
// This allows O(n) multi-pattern matching instead of O(n * patterns).
//
// The trie is lazily rebuilt when the number of tracked secrets changes.
type ahocorasickMatcher struct {
	provider ports.SensitiveValueProvider
	trie     *ahocorasick.Trie
	lastHash uint64
	mu       sync.RWMutex
}

// NewAhoCorasickMatcher creates a new matcher backed by a SensitiveValueProvider.
func NewAhoCorasickMatcher(provider ports.SensitiveValueProvider) SensitiveStringMatcher {
	return &ahocorasickMatcher{
		provider: provider,
	}
}

// FindAll returns all match positions for sensitive values in the input.
// Matches may overlap and are not sorted - use SortAndDeduplicateMatches to process them.
func (m *ahocorasickMatcher) FindAll(input string) []Match {
	if input == "" {
		return nil
	}

	trie := m.getTrie()
	if trie == nil {
		return nil
	}

	acMatches := trie.MatchString(input)
	if len(acMatches) == 0 {
		return nil
	}

	result := make([]Match, 0, len(acMatches))
	for _, acm := range acMatches {
		pos := int(acm.Pos())
		secret := acm.MatchString()
		result = append(result, Match{
			Start:  pos,
			End:    pos + len(secret),
			Secret: secret,
		})
	}

	return result
}

// ReplaceAll finds all occurrences of sensitive values and replaces them in a single pass.
// Uses FindAll internally, then deduplicates and applies replacements.
func (m *ahocorasickMatcher) ReplaceAll(input string, replacement func(secret string) string) string {
	matches := m.FindAll(input)
	if len(matches) == 0 {
		return input
	}

	deduped := SortAndDeduplicateMatches(matches)
	return ApplyReplacements(input, deduped, replacement)
}

// getTrie returns the current Aho-Corasick trie, rebuilding if necessary.
func (m *ahocorasickMatcher) getTrie() *ahocorasick.Trie {
	// Fast path: read lock to check if rebuild is needed
	m.mu.RLock()
	secrets := m.provider.AllValues()
	currentHash := computeHash(secrets)

	if m.trie != nil && currentHash == m.lastHash {
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
	currentHash = computeHash(secrets)

	if m.trie != nil && currentHash == m.lastHash {
		return m.trie
	}

	// Rebuild trie
	if len(secrets) == 0 {
		m.trie = nil
		m.lastHash = 0
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
		m.lastHash = 0 // Invalidate hash essentially
		return nil
	}

	m.trie = ahocorasick.NewTrieBuilder().AddStrings(filtered).Build()
	m.lastHash = currentHash
	return m.trie
}

// computeHash calculates a checksum of the secrets list.
// The order matters, ensuring that if secrets are reordered it is considered a change
// unless we sorted them first. However, since we want to respond to any change in the provider's
// returned slice, we'll hash them as returned.
func computeHash(secrets []string) uint64 {
	if len(secrets) == 0 {
		return 0
	}

	// Use ECMA polynomial
	table := crc64.MakeTable(crc64.ECMA)
	h := crc64.New(table)

	// Sort to ensure consistent hash regardless of provider order,
	// assuming the matcher doesn't care about order (Aho-Corasick doesn't).
	// This prevents unnecessary rebuilds if the provider returns randomized order but same set.
	// But first, let's copy to avoid modifying the input slice which might be owned by provider.
	// Wait, provider.AllValues() returns a copy or a fresh slice ideally.
	// The provider contract says "Return a copy".
	// Let's optimize: sorting secrets is good practice for the hash stability.

	sorted := make([]string, len(secrets))
	copy(sorted, secrets)
	sort.Strings(sorted)

	for _, s := range sorted {
		_, _ = io.WriteString(h, s)
		// Add a separator to distinguish ["ab", "c"] from ["a", "bc"]
		_, _ = io.WriteString(h, "\x00")
	}

	return h.Sum64()
}
