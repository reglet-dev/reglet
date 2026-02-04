package services

import (
	"context"
	"fmt"
	"time"

	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/domain/entities"
)

// LockfileService orchestrates plugin version resolution and locking.
type LockfileService struct {
	repo     ports.LockfileRepository
	resolver ports.VersionResolver
	digester ports.PluginDigester
}

// NewLockfileService creates a new LockfileService.
func NewLockfileService(
	repo ports.LockfileRepository,
	resolver ports.VersionResolver,
	digester ports.PluginDigester,
) *LockfileService {
	return &LockfileService{
		repo:     repo,
		resolver: resolver,
		digester: digester,
	}
}

// ResolvePlugins resolves plugin versions using the lockfile if available,
// or falls back to resolving constraints and updating the lockfile.
func (s *LockfileService) ResolvePlugins(
	ctx context.Context,
	profile *entities.Profile,
	lockfilePath string,
) (*entities.Lockfile, error) {
	// 1. Load existing lockfile
	lock, err := s.repo.Load(ctx, lockfilePath)
	if err != nil {
		return nil, fmt.Errorf("loading lockfile: %w", err)
	}

	if lock == nil {
		lock = entities.NewLockfile()
	}

	// 2. Resolve each plugin in the profile
	updated := false
	for _, pluginDecl := range profile.Plugins {
		spec, err := entities.ParsePluginDeclaration(pluginDecl)
		if err != nil {
			return nil, fmt.Errorf("parsing plugin declaration %q: %w", pluginDecl, err)
		}

		name := spec.Name
		constraint := spec.Version
		if constraint == "" {
			constraint = "latest" // Default if no version specified
		}

		// Check if locked
		locked := lock.GetPlugin(name)
		if locked != nil {
			if locked.Requested == constraint {
				continue // Already satisfied
			}
			// If constraint changed, we need to re-resolve
		}

		updated = true
		// Mock logic for "available" - in real code this comes from registry
		// For now we'll just lock the constraint as the version if it looks exact.
		resolvedVersion := constraint // Fallback

		// Update lock
		newLock := entities.PluginLock{
			Requested: constraint,
			Resolved:  resolvedVersion,
			Source:    spec.Source,
			Digest:    "sha256:placeholder", // Placeholder until we have digester integrated
			Fetched:   time.Now().UTC(),
		}

		if err := lock.AddPlugin(name, newLock); err != nil {
			return nil, err
		}
	}

	// 3. Save if updated
	if updated {
		lock.Generated = time.Now().UTC()
		if err := s.repo.Save(ctx, lock, lockfilePath); err != nil {
			return nil, fmt.Errorf("saving lockfile: %w", err)
		}
	}

	return lock, nil
}

// LockProfile adds a remote profile to the lockfile with its resolved version and digest.
// This enables reproducible builds by pinning profile versions.
func (s *LockfileService) LockProfile(
	ctx context.Context,
	lockfilePath string,
	profileURL string,
	version string,
	digest string,
) error {
	// Load existing lockfile
	lock, err := s.repo.Load(ctx, lockfilePath)
	if err != nil {
		return fmt.Errorf("loading lockfile: %w", err)
	}

	if lock == nil {
		lock = entities.NewLockfile()
	}

	// Ensure Profiles map is initialized
	if lock.Profiles == nil {
		lock.Profiles = make(map[string]entities.ProfileLock)
	}

	// Add or update the profile lock
	profileLock := entities.ProfileLock{
		Requested: profileURL,
		Resolved:  version,
		Source:    profileURL,
		Digest:    digest,
		Fetched:   time.Now().UTC(),
	}

	if err := lock.AddProfile(profileURL, profileLock); err != nil {
		return fmt.Errorf("adding profile lock: %w", err)
	}

	// Save updated lockfile
	lock.Generated = time.Now().UTC()
	lock.Version = 2 // Profile locking requires version 2
	if err := s.repo.Save(ctx, lock, lockfilePath); err != nil {
		return fmt.Errorf("saving lockfile: %w", err)
	}

	return nil
}

// GetLockedProfile retrieves a locked profile entry by URL.
// Returns nil if the profile is not locked.
func (s *LockfileService) GetLockedProfile(
	ctx context.Context,
	lockfilePath string,
	profileURL string,
) (*entities.ProfileLock, error) {
	lock, err := s.repo.Load(ctx, lockfilePath)
	if err != nil {
		return nil, fmt.Errorf("loading lockfile: %w", err)
	}

	if lock == nil {
		return nil, nil
	}

	return lock.GetProfile(profileURL), nil
}
