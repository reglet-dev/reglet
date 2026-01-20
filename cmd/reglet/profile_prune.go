// Package main provides the reglet CLI for compliance and infrastructure validation.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/reglet-dev/reglet/internal/infrastructure/profiles"
	"github.com/spf13/cobra"
)

func init() {
	profileCmd.AddCommand(newProfilePruneCmd())
}

// ProfilePruneOptions holds configuration for the profile prune command.
type ProfilePruneOptions struct {
	CommonOptions
	dryRun bool
	all    bool
}

func newProfilePruneCmd() *cobra.Command {
	opts := &ProfilePruneOptions{
		CommonOptions: DefaultCommonOptions(),
	}

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove stale profiles from cache",
		Long: `Remove expired or stale profiles from the local cache.

By default, only removes profiles that have exceeded their TTL.
Use --all to remove all cached profiles.`,
		Example: `  # Remove expired profiles
  reglet profile prune

  # Preview what would be removed
  reglet profile prune --dry-run

  # Remove all cached profiles
  reglet profile prune --all`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Quiet {
				quiet = true
				setupLogging()
			} else if opts.Verbose {
				logLevel = "debug"
				setupLogging()
			}

			return runProfilePruneAction(cmd.Context(), opts)
		},
	}

	opts.RegisterFlags(cmd)
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Preview what would be removed without deleting")
	cmd.Flags().BoolVar(&opts.all, "all", false, "Remove all cached profiles, not just expired ones")

	return cmd
}

func runProfilePruneAction(ctx context.Context, opts *ProfilePruneOptions) error {
	// Get cache directory path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	cacheDir := filepath.Join(homeDir, ".reglet", "profiles")

	// Check if cache directory exists
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		fmt.Println("No cached profiles found.")
		return nil
	}

	// Create cache repository
	cacheRepo, err := profiles.NewFSProfileCacheRepository(cacheDir)
	if err != nil {
		return fmt.Errorf("failed to create cache repository: %w", err)
	}

	// List all cached profiles
	entries, err := cacheRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list cached profiles: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No cached profiles found.")
		return nil
	}

	// Determine what to prune
	type pruneEntry struct {
		ref      string
		cacheKey string
		status   string
		size     int64
	}
	var toPrune []pruneEntry
	var totalSize int64

	for _, entry := range entries {
		shouldPrune := opts.all || !entry.IsFresh()

		if shouldPrune {
			status := "expired"
			if opts.all && entry.IsFresh() {
				status = "forced"
			}
			toPrune = append(toPrune, pruneEntry{
				ref:      entry.Reference().String(),
				cacheKey: entry.Reference().CacheKey(),
				size:     entry.Size(),
				status:   status,
			})
			totalSize += entry.Size()
		}
	}

	if len(toPrune) == 0 {
		fmt.Println("No profiles to prune.")
		return nil
	}

	// Preview or execute
	if opts.dryRun {
		fmt.Printf("Would remove %d profile(s) (%s):\n", len(toPrune), formatSize(totalSize))
		for _, p := range toPrune {
			fmt.Printf("  - %s (%s)\n", truncateURL(p.ref, 50), p.status)
		}
		return nil
	}

	// Actually prune by removing cache directories
	pruned := 0
	for _, p := range toPrune {
		cacheDir := filepath.Join(homeDir, ".reglet", "profiles", p.cacheKey)
		if err := os.RemoveAll(cacheDir); err != nil {
			fmt.Printf("Warning: failed to delete %s: %v\n", truncateURL(p.ref, 40), err)
		} else {
			pruned++
		}
	}

	fmt.Printf("✓ Pruned %d profile(s), freed %s\n", pruned, formatSize(totalSize))

	return nil
}
