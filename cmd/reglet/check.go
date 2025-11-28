package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/jrose/reglet/internal/config"
	"github.com/jrose/reglet/internal/engine"
	"github.com/jrose/reglet/internal/output"
)

// runCheck implements the check command
func runCheck(ctx context.Context, args []string) error {
	// Create flag set for check command
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	formatFlag := fs.String("format", "table", "Output format: table, json, yaml")
	outputFlag := fs.String("output", "", "Output file path (default: stdout)")

	// Parse flags
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Get profile path from positional argument
	if fs.NArg() == 0 {
		return fmt.Errorf("no profile specified\n\nUsage: reglet check <profile.yaml> [flags]")
	}

	profilePath := fs.Arg(0)

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
	if *outputFlag != "" {
		file, err := os.Create(*outputFlag)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()
		writer = file
		slog.Info("writing output", "file", *outputFlag, "format", *formatFlag)
	}

	// Format and output results
	if err := formatOutput(writer, result, *formatFlag); err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	// Return non-zero exit code if there were failures or errors
	if result.Summary.FailedControls > 0 || result.Summary.ErrorControls > 0 {
		os.Exit(1)
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
