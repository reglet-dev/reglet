package services

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/reglet-dev/reglet/internal/domain/execution"
	"github.com/reglet-dev/reglet/internal/domain/values"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStatusAggregator_ExpressionCaching verifies that expressions are cached and reused
func TestStatusAggregator_ExpressionCaching(t *testing.T) {
	aggregator := NewStatusAggregator()

	evidence := &execution.Evidence{
		Status: true,
		Data: map[string]interface{}{
			"value": 42,
		},
	}

	expects := []string{"data.value == 42"}

	// First call - should compile and cache
	status1, results1 := aggregator.DetermineObservationStatus(context.Background(), evidence, expects)
	require.Len(t, results1, 1)
	assert.True(t, results1[0].Passed)

	// Verify expression was cached
	aggregator.cacheMu.RLock()
	_, cached := aggregator.programCache["data.value == 42"]
	aggregator.cacheMu.RUnlock()
	assert.True(t, cached, "Expression should be cached after first use")

	// Second call with same expression - should use cache
	status2, results2 := aggregator.DetermineObservationStatus(context.Background(), evidence, expects)
	require.Len(t, results2, 1)
	assert.True(t, results2[0].Passed)
	assert.Equal(t, status1, status2)

	// Verify cache size hasn't changed (no duplicate entries)
	aggregator.cacheMu.RLock()
	cacheSize := len(aggregator.programCache)
	aggregator.cacheMu.RUnlock()
	assert.Equal(t, 1, cacheSize, "Cache should contain exactly one entry")
}

// TestStatusAggregator_CacheGrowth verifies cache grows with unique expressions
func TestStatusAggregator_CacheGrowth(t *testing.T) {
	aggregator := NewStatusAggregator()

	evidence := &execution.Evidence{
		Status: true,
		Data: map[string]interface{}{
			"a": 1,
			"b": 2,
			"c": 3,
		},
	}

	// Evaluate different expressions
	expressions := [][]string{
		{"data.a == 1"},
		{"data.b == 2"},
		{"data.c == 3"},
	}

	for _, expects := range expressions {
		aggregator.DetermineObservationStatus(context.Background(), evidence, expects)
	}

	// Verify all expressions were cached
	aggregator.cacheMu.RLock()
	cacheSize := len(aggregator.programCache)
	aggregator.cacheMu.RUnlock()

	assert.Equal(t, 3, cacheSize, "Cache should contain all unique expressions")
}

// TestStatusAggregator_ConcurrentCaching verifies thread-safe cache access
func TestStatusAggregator_ConcurrentCaching(t *testing.T) {
	aggregator := NewStatusAggregator()

	evidence := &execution.Evidence{
		Status: true,
		Data: map[string]interface{}{
			"value": 100,
		},
	}

	expects := []string{"data.value > 50"}

	// Run many concurrent evaluations with the same expression
	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Channel to collect errors
	errChan := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			status, results := aggregator.DetermineObservationStatus(context.Background(), evidence, expects)
			if len(results) != 1 {
				errChan <- assert.AnError
				return
			}
			if !results[0].Passed {
				errChan <- assert.AnError
				return
			}
			if status != values.StatusPass {
				errChan <- assert.AnError
			}
		}()
	}

	wg.Wait()
	close(errChan)

	// Check no errors occurred
	for err := range errChan {
		t.Fatalf("Concurrent execution error: %v", err)
	}

	// Verify expression was only compiled once
	aggregator.cacheMu.RLock()
	cacheSize := len(aggregator.programCache)
	aggregator.cacheMu.RUnlock()

	assert.Equal(t, 1, cacheSize, "Expression should be cached exactly once despite concurrent access")
}

// TestStatusAggregator_CacheWithMultipleExpressions verifies caching with multiple expect expressions
func TestStatusAggregator_CacheWithMultipleExpressions(t *testing.T) {
	aggregator := NewStatusAggregator()

	evidence := &execution.Evidence{
		Status: true,
		Data: map[string]interface{}{
			"size":        1024,
			"permissions": "0644",
			"owner":       "root",
		},
	}

	expects := []string{
		"data.size > 100",
		"data.permissions == '0644'",
		"data.owner == 'root'",
	}

	// First evaluation
	status, results := aggregator.DetermineObservationStatus(context.Background(), evidence, expects)
	require.Len(t, results, 3)
	assert.Equal(t, values.StatusPass, status)

	// Verify all expressions were cached
	aggregator.cacheMu.RLock()
	cacheSize := len(aggregator.programCache)
	aggregator.cacheMu.RUnlock()
	assert.Equal(t, 3, cacheSize, "All three expressions should be cached")

	// Second evaluation - should use cache for all
	status2, results2 := aggregator.DetermineObservationStatus(context.Background(), evidence, expects)
	require.Len(t, results2, 3)
	assert.Equal(t, status, status2)

	// Verify cache size unchanged (no duplicates)
	aggregator.cacheMu.RLock()
	cacheSize2 := len(aggregator.programCache)
	aggregator.cacheMu.RUnlock()
	assert.Equal(t, 3, cacheSize2, "Cache size should remain unchanged on reuse")
}

// TestStatusAggregator_CacheInvalidExpression verifies invalid expressions aren't cached
func TestStatusAggregator_CacheInvalidExpression(t *testing.T) {
	aggregator := NewStatusAggregator()

	evidence := &execution.Evidence{
		Status: true,
		Data:   map[string]interface{}{},
	}

	invalidExpects := []string{"data.value == !!!INVALID!!!"}

	// Evaluate invalid expression
	status, results := aggregator.DetermineObservationStatus(context.Background(), evidence, invalidExpects)
	require.Len(t, results, 1)
	assert.False(t, results[0].Passed)
	assert.Equal(t, values.StatusError, status)

	// Verify invalid expression is NOT cached
	aggregator.cacheMu.RLock()
	_, cached := aggregator.programCache["data.value == !!!INVALID!!!"]
	cacheSize := len(aggregator.programCache)
	aggregator.cacheMu.RUnlock()

	assert.False(t, cached, "Invalid expressions should not be cached")
	assert.Equal(t, 0, cacheSize, "Cache should be empty for failed compilations")
}

// BenchmarkStatusAggregator_WithCache benchmarks expression evaluation with caching
func BenchmarkStatusAggregator_WithCache(b *testing.B) {
	aggregator := NewStatusAggregator()

	evidence := &execution.Evidence{
		Status: true,
		Data: map[string]interface{}{
			"size": 1024,
		},
	}

	expects := []string{"data.size > 100", "data.size < 2000"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		aggregator.DetermineObservationStatus(context.Background(), evidence, expects)
	}
}

// BenchmarkStatusAggregator_WithoutCache benchmarks expression evaluation without caching
// (by using a fresh aggregator each time to prevent caching)
func BenchmarkStatusAggregator_WithoutCache(b *testing.B) {
	evidence := &execution.Evidence{
		Status: true,
		Data: map[string]interface{}{
			"size": 1024,
		},
	}

	expects := []string{"data.size > 100", "data.size < 2000"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create new aggregator each time to prevent caching
		aggregator := NewStatusAggregator()
		aggregator.DetermineObservationStatus(context.Background(), evidence, expects)
	}
}

// BenchmarkStatusAggregator_ConcurrentAccess benchmarks concurrent cache access
func BenchmarkStatusAggregator_ConcurrentAccess(b *testing.B) {
	aggregator := NewStatusAggregator()

	evidence := &execution.Evidence{
		Status: true,
		Data: map[string]interface{}{
			"value": 42,
		},
	}

	expects := []string{"data.value == 42"}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			aggregator.DetermineObservationStatus(context.Background(), evidence, expects)
		}
	})
}

// TestStatusAggregator_CachePerformance measures actual cache performance improvement
func TestStatusAggregator_CachePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	aggregator := NewStatusAggregator()

	evidence := &execution.Evidence{
		Status: true,
		Data: map[string]interface{}{
			"value": 42,
		},
	}

	expects := []string{"data.value == 42"}

	// Measure time for first call (compilation + execution)
	start1 := time.Now()
	aggregator.DetermineObservationStatus(context.Background(), evidence, expects)
	duration1 := time.Since(start1)

	// Measure time for subsequent call (cache hit + execution)
	start2 := time.Now()
	aggregator.DetermineObservationStatus(context.Background(), evidence, expects)
	duration2 := time.Since(start2)

	// Cached call should be faster (though not always guaranteed on first run due to warmup)
	t.Logf("First call (compile): %v", duration1)
	t.Logf("Second call (cached): %v", duration2)
	t.Logf("Speedup: %.2fx", float64(duration1)/float64(duration2))
}
