package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/reglet-dev/reglet/internal/application/services"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockRepo implements ports.LockfileRepository
type MockRepo struct {
	mock.Mock
}

func (m *MockRepo) Load(ctx context.Context, path string) (*entities.Lockfile, error) {
	args := m.Called(ctx, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.Lockfile), args.Error(1)
}

func (m *MockRepo) Save(ctx context.Context, lockfile *entities.Lockfile, path string) error {
	args := m.Called(ctx, lockfile, path)
	return args.Error(0)
}

func (m *MockRepo) Exists(ctx context.Context, path string) (bool, error) {
	args := m.Called(ctx, path)
	return args.Bool(0), args.Error(1)
}

func TestLockfileService_ResolvePlugins(t *testing.T) {
	t.Parallel()

	// Setup
	mockRepo := new(MockRepo)
	svc := services.NewLockfileService(mockRepo, nil, nil) // Resolver/Digester unused for now

	ctx := context.Background()
	lockPath := "reglet.lock"

	t.Run("creates new lockfile if missing", func(t *testing.T) {
		// Expect loading returns nil (missing)
		mockRepo.On("Load", ctx, lockPath).Return(nil, nil).Once()
		// Expect save with new lockfile
		mockRepo.On("Save", ctx, mock.AnythingOfType("*entities.Lockfile"), lockPath).Return(nil).Once()

		profile := &entities.Profile{
			Plugins: []string{"reglet/test@1.0"},
		}

		lock, err := svc.ResolvePlugins(ctx, profile, lockPath)
		require.NoError(t, err)
		require.NotNil(t, lock)
		assert.Equal(t, 1, lock.PluginCount())

		plugin := lock.GetPlugin("test") // "reglet/test" -> name="test"
		require.NotNil(t, plugin)
		assert.Equal(t, "1.0", plugin.Requested)

		mockRepo.AssertExpectations(t)
	})

	t.Run("uses existing locked version", func(t *testing.T) {
		existingLock := entities.NewLockfile()
		existingLock.AddPlugin("test", entities.PluginLock{
			Requested: "1.0",
			Resolved:  "1.0.0",
			Digest:    "sha256:old",
			Fetched:   time.Now(),
		})

		// Expect loading returns existing
		mockRepo.On("Load", ctx, lockPath).Return(existingLock, nil).Once()
		// Should NOT save because nothing changed
		// (implicit: strictly checking calls)

		profile := &entities.Profile{
			Plugins: []string{"reglet/test@1.0"},
		}

		lock, err := svc.ResolvePlugins(ctx, profile, lockPath)
		require.NoError(t, err)
		assert.Equal(t, "1.0.0", lock.GetPlugin("test").Resolved)

		mockRepo.AssertNotCalled(t, "Save")
	})

	t.Run("updates lock on version change", func(t *testing.T) {
		existingLock := entities.NewLockfile()
		existingLock.AddPlugin("test", entities.PluginLock{
			Requested: "1.0", // Old constraint
			Resolved:  "1.0.0",
			Digest:    "sha256:old",
		})

		mockRepo.On("Load", ctx, lockPath).Return(existingLock, nil).Once()
		mockRepo.On("Save", ctx, mock.MatchedBy(func(l *entities.Lockfile) bool {
			return l.GetPlugin("test").Requested == "2.0"
		}), lockPath).Return(nil).Once()

		profile := &entities.Profile{
			Plugins: []string{"reglet/test@2.0"}, // New constraint
		}

		lock, err := svc.ResolvePlugins(ctx, profile, lockPath)
		require.NoError(t, err)
		assert.Equal(t, "2.0", lock.GetPlugin("test").Requested)
	})
}
