// Package main provides the reglet CLI for compliance and infrastructure validation.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/infrastructure/profiles"
	"github.com/spf13/cobra"
)

func init() {
	// Register under the profile parent command
	profileCmd.AddCommand(newProfileOutdatedCmd())
}

// ProfileOutdatedOptions holds configuration for the profile outdated command.
type ProfileOutdatedOptions struct {
	CommonOptions
}

func newProfileOutdatedCmd() *cobra.Command {
	opts := &ProfileOutdatedOptions{
		CommonOptions: DefaultCommonOptions(),
	}

	cmd := &cobra.Command{
		Use:   "outdated",
		Short: "Check for updates to cached remote profiles",
		Long: `Check if any cached remote profiles have newer versions available upstream.

This command performs lightweight HEAD requests to compare ETags without
downloading the full profile content. Use --refresh flag with 'reglet check'
to update to the latest version.`,
		Example: `  # Check all cached profiles for updates
  reglet profile outdated

  # Show detailed output
  reglet profile outdated --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Quiet {
				quiet = true
				setupLogging()
			} else if opts.Verbose {
				logLevel = "debug"
				setupLogging()
			}

			return runProfileOutdatedAction(cmd.Context(), opts)
		},
	}

	opts.RegisterFlags(cmd)

	return cmd
}

func runProfileOutdatedAction(ctx context.Context, opts *ProfileOutdatedOptions) error {
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

	// Create HTTP fetcher for update checks
	fetcher := profiles.NewHTTPProfileFetcher()

	// Create cache repository to list cached profiles
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

	// Check each profile for updates
	type updateStatus struct {
		Error      error
		URL        string
		CurrentTag string
		RemoteTag  string
		HasUpdate  bool
	}

	var results []updateStatus
	var updatesAvailable int

	for _, entry := range entries {
		ref := entry.Reference()

		// Only check HTTPS profiles
		if !ref.IsHTTPS() {
			continue
		}

		result, err := fetcher.CheckForUpdate(ctx, ref, entry.ETag(), ports.FetchOptions{})
		if err != nil {
			results = append(results, updateStatus{
				URL:   ref.String(),
				Error: fmt.Errorf("check failed: %w", err),
			})
			continue
		}

		if result.HasUpdate {
			updatesAvailable++
		}

		results = append(results, updateStatus{
			URL:        ref.String(),
			HasUpdate:  result.HasUpdate,
			CurrentTag: result.CurrentETag,
			RemoteTag:  result.RemoteETag,
		})
	}

	// Print results
	if opts.Format == "json" {
		// JSON output
		_, _ = fmt.Printf(`{"profiles_checked": %d, "updates_available": %d}`, len(results), updatesAvailable)
		_, _ = fmt.Println()
		return nil
	}

	// Table output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "STATUS\tURL")
	_, _ = fmt.Fprintln(w, "------\t---")

	for _, r := range results {
		status := "✓ up-to-date"
		if r.Error != nil {
			status = "✗ " + r.Error.Error()
		} else if r.HasUpdate {
			status = "↑ update available"
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\n", status, truncateURL(r.URL, 60))
	}

	if err := w.Flush(); err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("Checked %d profile(s), %d update(s) available.\n", len(results), updatesAvailable)

	if updatesAvailable > 0 {
		fmt.Println("\nTo update, use: reglet check <profile-url> --refresh")
	}

	return nil
}

// truncateURL shortens a URL for display.
func truncateURL(url string, maxLen int) string {
	if len(url) <= maxLen {
		return url
	}
	return url[:maxLen-3] + "..."
}
