// Package memory provides in-memory implementations of domain repositories.
package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/reglet-dev/reglet/internal/domain/execution"
	"github.com/reglet-dev/reglet/internal/domain/repositories"
)

// Ensure interface compliance
var _ repositories.ExecutionResultRepository = (*ExecutionResultRepository)(nil)

// ExecutionResultRepository is an in-memory implementation of ExecutionResultRepository.
// Useful for testing and ephemeral storage.
type ExecutionResultRepository struct {
	results map[uuid.UUID]*execution.ExecutionResult
	mu      sync.RWMutex
}

// NewExecutionResultRepository creates a new in-memory repository.
func NewExecutionResultRepository() *ExecutionResultRepository {
	return &ExecutionResultRepository{
		results: make(map[uuid.UUID]*execution.ExecutionResult),
	}
}

// Save persists an execution result.
func (r *ExecutionResultRepository) Save(_ context.Context, result *execution.ExecutionResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Store the result
	// In a real DB, we'd serialize here. In memory, we store the pointer.
	// Callers should not modify the result after saving if they want consistency.
	r.results[result.GetID().UUID()] = result
	return nil
}

// FindByID retrieves an execution result by its unique ID.
func (r *ExecutionResultRepository) FindByID(_ context.Context, id uuid.UUID) (*execution.ExecutionResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result, ok := r.results[id]
	if !ok {
		return nil, fmt.Errorf("execution result not found: %s", id)
	}
	return result, nil
}

// FindByProfile retrieves recent execution results for a specific profile.
func (r *ExecutionResultRepository) FindByProfile(_ context.Context, profileName string, limit int) ([]*execution.ExecutionResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matches []*execution.ExecutionResult
	for _, res := range r.results {
		if res.ProfileName == profileName {
			matches = append(matches, res)
		}
	}

	// Sort by start time descending (newest first)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].StartTime.After(matches[j].StartTime)
	})

	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}

	return matches, nil
}

// FindBetween retrieves execution results for a profile within a time range.
func (r *ExecutionResultRepository) FindBetween(_ context.Context, profileName string, start, end time.Time) ([]*execution.ExecutionResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matches []*execution.ExecutionResult
	for _, res := range r.results {
		if res.ProfileName == profileName {
			// Check if start time is within range [start, end]
			if (res.StartTime.Equal(start) || res.StartTime.After(start)) &&
				(res.StartTime.Equal(end) || res.StartTime.Before(end)) {
				matches = append(matches, res)
			}
		}
	}

	// Sort by start time descending
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].StartTime.After(matches[j].StartTime)
	})

	return matches, nil
}
