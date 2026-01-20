// Package main provides the reglet CLI for compliance and infrastructure validation.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/reglet-dev/reglet/internal/infrastructure/profiles"
	"github.com/spf13/cobra"
)

func init() {
	profileCmd.AddCommand(newProfileListCmd())
}

// ProfileListOptions holds configuration for the profile list command.
type ProfileListOptions struct {
	CommonOptions
}

func newProfileListCmd() *cobra.Command {
	opts := &ProfileListOptions{
		CommonOptions: DefaultCommonOptions(),
	}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List cached remote profiles",
		Long: `List all remote profiles that have been cached locally.

Shows the profile URL, version, size, and when it was last fetched.`,
		Example: `  # List all cached profiles
  reglet profile list

  # Show as JSON
  reglet profile list --format json`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Quiet {
				quiet = true
				setupLogging()
			} else if opts.Verbose {
				logLevel = "debug"
				setupLogging()
			}

			return runProfileListAction(cmd.Context(), opts)
		},
	}

	opts.RegisterFlags(cmd)

	return cmd
}

func runProfileListAction(ctx context.Context, opts *ProfileListOptions) error {
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

	// Print results
	if opts.Format == "json" {
		fmt.Printf(`{"cached_profiles": %d}`, len(entries))
		fmt.Println()
		return nil
	}

	// Table output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "URL\tVERSION\tSIZE\tAGE")
	_, _ = fmt.Fprintln(w, "---\t-------\t----\t---")

	for _, entry := range entries {
		ref := entry.Reference()
		version := ref.Version()
		if version == "" {
			version = "latest"
		}

		age := time.Since(entry.FetchedAt())
		ageStr := formatAge(age)

		sizeStr := formatSize(entry.Size())

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			truncateURL(ref.String(), 50),
			version,
			sizeStr,
			ageStr,
		)
	}

	if err := w.Flush(); err != nil {
		return err
	}

	fmt.Printf("\nTotal: %d cached profile(s)\n", len(entries))

	return nil
}

// formatAge formats a duration as a human-readable age string.
func formatAge(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		return fmt.Sprintf("%dm ago", mins)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		return fmt.Sprintf("%dh ago", hours)
	}
	days := int(d.Hours() / 24)
	return fmt.Sprintf("%dd ago", days)
}

// formatSize formats bytes as a human-readable size string.
func formatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1fMB", float64(bytes)/(1024*1024))
}
