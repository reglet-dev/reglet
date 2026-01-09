package memory

import (
	"context"
	"testing"
	"time"

	"github.com/reglet-dev/reglet/internal/domain/execution"
	"github.com/reglet-dev/reglet/internal/domain/values"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryExecutionResultRepository_SaveAndFind(t *testing.T) {
	repo := NewExecutionResultRepository()
	ctx := context.Background()

	id := values.NewExecutionID()
	result := execution.NewExecutionResultWithID(id, "test-profile", "1.0.0")

	// Save
	err := repo.Save(ctx, result)
	require.NoError(t, err)

	// FindByID
	found, err := repo.FindByID(ctx, id.UUID())
	require.NoError(t, err)
	assert.Equal(t, result.ProfileName, found.ProfileName)
	assert.True(t, found.GetID().Equals(id))

	// FindByID Not Found
	_, err = repo.FindByID(ctx, values.NewExecutionID().UUID())
	assert.Error(t, err)
}

func TestMemoryExecutionResultRepository_FindByProfile(t *testing.T) {
	repo := NewExecutionResultRepository()
	ctx := context.Background()

	// Create 3 results for "profile-a" with different times
	now := time.Now()
	r1 := execution.NewExecutionResult("profile-a", "1.0")
	r1.StartTime = now.Add(-3 * time.Hour)
	r2 := execution.NewExecutionResult("profile-a", "1.0")
	r2.StartTime = now.Add(-2 * time.Hour)
	r3 := execution.NewExecutionResult("profile-a", "1.0")
	r3.StartTime = now.Add(-1 * time.Hour)

	// Create 1 result for "profile-b"
	r4 := execution.NewExecutionResult("profile-b", "1.0")

	require.NoError(t, repo.Save(ctx, r1))
	require.NoError(t, repo.Save(ctx, r2))
	require.NoError(t, repo.Save(ctx, r3))
	require.NoError(t, repo.Save(ctx, r4))

	// Test FindByProfile limit
	results, err := repo.FindByProfile(ctx, "profile-a", 2)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, r3.GetID(), results[0].GetID()) // Newest first
	assert.Equal(t, r2.GetID(), results[1].GetID())

	// Test FindByProfile all
	results, err = repo.FindByProfile(ctx, "profile-a", 0)
	require.NoError(t, err)
	require.Len(t, results, 3)
}

func TestMemoryExecutionResultRepository_FindBetween(t *testing.T) {
	repo := NewExecutionResultRepository()
	ctx := context.Background()

	now := time.Now()
	// Target window: [now-2h, now-1h]
	start := now.Add(-2 * time.Hour)
	end := now.Add(-1 * time.Hour)

	r1 := execution.NewExecutionResult("profile-a", "1.0")
	r1.StartTime = now.Add(-3 * time.Hour) // Before window

	r2 := execution.NewExecutionResult("profile-a", "1.0")
	r2.StartTime = now.Add(-90 * time.Minute) // In window

	r3 := execution.NewExecutionResult("profile-a", "1.0")
	r3.StartTime = now.Add(-30 * time.Minute) // After window

	require.NoError(t, repo.Save(ctx, r1))
	require.NoError(t, repo.Save(ctx, r2))
	require.NoError(t, repo.Save(ctx, r3))

	results, err := repo.FindBetween(ctx, "profile-a", start, end)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, r2.GetID(), results[0].GetID())
}
