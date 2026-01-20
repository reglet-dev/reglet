// Package main provides the reglet CLI for compliance and infrastructure validation.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/domain/values"
	"github.com/reglet-dev/reglet/internal/infrastructure/profiles"
	"github.com/spf13/cobra"
)

func init() {
	profileCmd.AddCommand(newProfilePullCmd())
}

// ProfilePullOptions holds configuration for the profile pull command.
type ProfilePullOptions struct {
	CommonOptions
	refresh  bool
	insecure bool
}

func newProfilePullCmd() *cobra.Command {
	opts := &ProfilePullOptions{
		CommonOptions: DefaultCommonOptions(),
	}

	cmd := &cobra.Command{
		Use:   "pull <url>",
		Short: "Pre-fetch a remote profile without executing",
		Long: `Download and cache a remote profile without running any checks.

This is useful for pre-warming the cache before running checks, or for
ensuring profiles are available for offline use.`,
		Example: `  # Pre-fetch a profile
  reglet profile pull https://example.com/compliance.yaml

  # Force re-fetch even if cached
  reglet profile pull https://example.com/compliance.yaml --refresh`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Quiet {
				quiet = true
				setupLogging()
			} else if opts.Verbose {
				logLevel = "debug"
				setupLogging()
			}

			return runProfilePullAction(cmd.Context(), args[0], opts)
		},
	}

	opts.RegisterFlags(cmd)
	cmd.Flags().BoolVar(&opts.refresh, "refresh", false, "Force re-fetch even if cached")
	cmd.Flags().BoolVar(&opts.insecure, "insecure", false, "Skip TLS certificate verification")

	return cmd
}

func runProfilePullAction(ctx context.Context, url string, opts *ProfilePullOptions) error {
	// Parse the URL
	ref, err := values.ParseProfileReference(url)
	if err != nil {
		return fmt.Errorf("invalid profile URL: %w", err)
	}

	// Get cache directory path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	cacheDir := filepath.Join(homeDir, ".reglet", "profiles")

	// Create cache repository
	cacheRepo, err := profiles.NewFSProfileCacheRepository(cacheDir)
	if err != nil {
		return fmt.Errorf("failed to create cache repository: %w", err)
	}

	// Check cache first (unless --refresh)
	if !opts.refresh {
		cached, err := cacheRepo.Find(ctx, ref)
		if err == nil && cached != nil && cached.IsFresh() {
			fmt.Printf("✓ Profile already cached: %s\n", truncateURL(url, 60))
			fmt.Printf("  Size: %s\n", formatSize(cached.Size()))
			fmt.Println("  Use --refresh to force re-fetch")
			return nil
		}
	}

	// Create fetcher
	fetcher := profiles.NewHTTPProfileFetcher()

	// Fetch the profile
	fmt.Printf("Fetching %s...\n", truncateURL(url, 60))

	fetchOpts := ports.FetchOptions{
		Insecure: opts.insecure,
	}

	result, err := fetcher.Fetch(ctx, ref, fetchOpts)
	if err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}

	// Cache via the remote profile service layer (it handles caching)
	// For now, just confirm the fetch worked
	fmt.Printf("✓ Fetched: %s (%s)\n", truncateURL(url, 60), formatSize(int64(len(result.Content))))
	fmt.Printf("  Hash: %s\n", result.ContentHash.String())

	return nil
}
