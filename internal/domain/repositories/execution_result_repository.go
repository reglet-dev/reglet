// Package repositories defines interfaces for domain persistence.
package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/whiskeyjimbo/reglet/internal/domain/execution"
)

// ExecutionResultRepository defines the interface for persisting execution results.
type ExecutionResultRepository interface {
	// Save persists an execution result.
	Save(ctx context.Context, result *execution.ExecutionResult) error

	// FindByID retrieves an execution result by its unique ID.
	FindByID(ctx context.Context, id uuid.UUID) (*execution.ExecutionResult, error)

	// FindByProfile retrieves recent execution results for a specific profile.
	FindByProfile(ctx context.Context, profileName string, limit int) ([]*execution.ExecutionResult, error)

	// FindBetween retrieves execution results for a profile within a time range.
	FindBetween(ctx context.Context, profileName string, start, end time.Time) ([]*execution.ExecutionResult, error)
}
