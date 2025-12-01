package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/jrose/reglet/internal/config"
	"github.com/jrose/reglet/internal/engine"
	"github.com/jrose/reglet/internal/output"
	"github.com/spf13/cobra"
)

var (
	format  string
	outFile string
)

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:   "check <profile.yaml>",
	Short: "Execute compliance checks from a profile",
	Long: `Load a profile configuration and execute the defined validation controls.
The profile must be a valid YAML file defining the checks to run.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCheckAction(cmd.Context(), args[0])
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)

	checkCmd.Flags().StringVar(&format, "format", "table", "Output format: table, json, yaml")
	checkCmd.Flags().StringVarP(&outFile, "output", "o", "", "Output file path (default: stdout)")
}

// runCheckAction implements the core logic for the check command
func runCheckAction(ctx context.Context, profilePath string) error {
	slog.Info("loading profile", "path", profilePath)

	// Load profile
	profile, err := config.LoadProfile(profilePath)
	if err != nil {
		return fmt.Errorf("failed to load profile: %w", err)
	}

	slog.Info("profile loaded", "name", profile.Metadata.Name, "version", profile.Metadata.Version)

	// Apply variable substitution
	if err := config.SubstituteVariables(profile); err != nil {
		return fmt.Errorf("failed to substitute variables: %w", err)
	}

	// Validate profile
	if err := config.Validate(profile); err != nil {
		return fmt.Errorf("profile validation failed: %w", err)
	}

	slog.Info("profile validated", "controls", len(profile.Controls.Items))

	// Create execution engine
	eng, err := engine.NewEngine(ctx)
	if err != nil {
		return fmt.Errorf("failed to create engine: %w", err)
	}
	defer eng.Close(ctx)

	slog.Info("executing profile")

	// Execute profile
	result, err := eng.Execute(ctx, profile)
	if err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}

	slog.Info("execution complete",
		"duration", result.Duration,
		"total_controls", result.Summary.TotalControls,
		"passed", result.Summary.PassedControls,
		"failed", result.Summary.FailedControls,
		"errors", result.Summary.ErrorControls)

	// Determine output writer
	writer := os.Stdout
	if outFile != "" {
		file, err := os.Create(outFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()
		writer = file
		slog.Info("writing output", "file", outFile, "format", format)
	}

	// Format and output results
	if err := formatOutput(writer, result, format); err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	// Return non-zero exit code if there were failures or errors
	// We return an error here to let Cobra/main handle the exit code,
	// or we can use os.Exit(1) if we want to force it immediately.
	// Cobra doesn't automatically exit 1 on RunE error unless we configure it,
	// but usually it prints the error.
	// However, for business logic failure (checks failed), we might not want to print "Error: ..."
	// but just exit with status 1.
	if result.Summary.FailedControls > 0 || result.Summary.ErrorControls > 0 {
		return fmt.Errorf("check failed: %d passed, %d failed, %d errors",
			result.Summary.PassedControls,
			result.Summary.FailedControls,
			result.Summary.ErrorControls)
	}

	return nil
}

// formatOutput formats the result using the specified formatter
func formatOutput(writer *os.File, result *engine.ExecutionResult, format string) error {
	switch format {
	case "table":
		formatter := output.NewTableFormatter(writer)
		return formatter.Format(result)
	case "json":
		formatter := output.NewJSONFormatter(writer, true) // Pretty-print JSON
		return formatter.Format(result)
	case "yaml":
		formatter := output.NewYAMLFormatter(writer)
		return formatter.Format(result)
	default:
		return fmt.Errorf("unknown format: %s (supported: table, json, yaml)", format)
	}
}