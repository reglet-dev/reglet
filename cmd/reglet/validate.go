// Package main provides the reglet CLI for compliance and infrastructure validation.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/reglet-dev/reglet/internal/application/dto"
	"github.com/reglet-dev/reglet/internal/infrastructure/container"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

// ValidateOptions holds configuration for the validate command.
type ValidateOptions struct {
	CommonOptions
	skipSchemaValidation bool
	skipExpectValidation bool
	showStats            bool
}

func init() {
	rootCmd.AddCommand(newValidateCmd())
}

func newValidateCmd() *cobra.Command {
	opts := &ValidateOptions{
		CommonOptions: DefaultCommonOptions(),
	}

	cmd := &cobra.Command{
		Use:   "validate <profile.yaml>",
		Short: "Validate profile schema without execution",
		Long: `Fast validation of profile YAML structure without running checks.

Validates:
  - Profile metadata (name, version)
  - Control definitions (ID, name, observations)
  - Plugin configuration structure
  - Dependency graph (cycle detection)
  - Expect expression syntax (expr-lang)

Use this for quick feedback during profile development.`,
		Example: `  # Validate a profile
  reglet validate profile.yaml

  # Validate with JSON output
  reglet validate profile.yaml --format json

  # Skip expect expression validation (faster)
  reglet validate profile.yaml --skip-expects

  # Show validation statistics
  reglet validate profile.yaml --stats`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.ValidateFlags(); err != nil {
				return err
			}

			if opts.Quiet {
				quiet = true
				setupLogging()
			} else if opts.Verbose {
				logLevel = "debug"
				setupLogging()
			}

			return runValidateAction(cmd.Context(), args[0], opts)
		},
	}

	// Register common flags
	opts.RegisterFlags(cmd)

	// Validate-specific flags
	cmd.Flags().BoolVar(&opts.skipSchemaValidation, "skip-schema", false,
		"Skip plugin config schema validation")
	cmd.Flags().BoolVar(&opts.skipExpectValidation, "skip-expects", false,
		"Skip expect expression syntax validation")
	cmd.Flags().BoolVar(&opts.showStats, "stats", false,
		"Show validation statistics")

	return cmd
}

func runValidateAction(ctx context.Context, profilePath string, opts *ValidateOptions) error {
	// 1. Initialize container
	c, err := container.New(container.Options{
		SystemConfigPath: cfgFile,
		Logger:           slog.Default(),
	})
	if err != nil {
		return fmt.Errorf("failed to initialize application: %w", err)
	}

	// 2. Build request
	request := dto.ValidateProfileRequest{
		ProfilePath:          profilePath,
		SkipSchemaValidation: opts.skipSchemaValidation,
		SkipExpectValidation: opts.skipExpectValidation,
		Metadata: dto.RequestMetadata{
			RequestID: generateRequestID(),
		},
	}

	// 3. Execute
	response, err := c.ValidateProfileUseCase().Execute(ctx, request)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// 4. Output results
	return printValidationResult(response, opts)
}

func printValidationResult(resp *dto.ValidateProfileResponse, opts *ValidateOptions) error {
	switch opts.Format {
	case "json":
		return printValidationJSON(resp)
	case "yaml":
		return printValidationYAML(resp)
	default:
		return printValidationTable(resp, opts)
	}
}

func printValidationTable(resp *dto.ValidateProfileResponse, opts *ValidateOptions) error {
	if resp.Valid {
		fmt.Printf("\n✓ Profile is valid: %s v%s\n", resp.ProfileName, resp.Version)
	} else {
		fmt.Printf("\n✗ Profile validation failed: %s v%s\n", resp.ProfileName, resp.Version)
		fmt.Println(strings.Repeat("─", 60))

		// Group errors by type
		errorsByType := make(map[string][]dto.ValidationError)
		for _, e := range resp.Errors {
			errorsByType[e.Type] = append(errorsByType[e.Type], e)
		}

		// Print errors by type
		titleCaser := cases.Title(language.English)
		for errType, errors := range errorsByType {
			fmt.Printf("\n%s errors:\n", titleCaser.String(errType))
			for _, e := range errors {
				fmt.Printf("  ✗ %s\n", e.Message)
				if e.Path != "" && e.Path != "profile" {
					fmt.Printf("    at: %s\n", e.Path)
				}
			}
		}
	}

	// Print warnings if any
	if len(resp.Warnings) > 0 {
		fmt.Println("\nWarnings:")
		for _, w := range resp.Warnings {
			fmt.Printf("  ⚠ %s\n", w)
		}
	}

	// Print stats if requested
	if opts.showStats || opts.Verbose {
		fmt.Println()
		fmt.Println(strings.Repeat("─", 60))
		fmt.Println("Statistics:")
		fmt.Printf("  Controls:     %d\n", resp.Stats.ControlCount)
		fmt.Printf("  Observations: %d\n", resp.Stats.ObservationCount)
		fmt.Printf("  Expectations: %d\n", resp.Stats.ExpectCount)
		if len(resp.Stats.PluginsUsed) > 0 {
			fmt.Printf("  Plugins:      %s\n", strings.Join(resp.Stats.PluginsUsed, ", "))
		}
	}

	fmt.Println()

	// Return error exit code if invalid
	if !resp.Valid {
		return fmt.Errorf("validation failed with %d error(s)", len(resp.Errors))
	}

	return nil
}

// validateOutputData is the structured output format for JSON/YAML.
type validateOutputData struct {
	ProfileName string                `json:"profile_name" yaml:"profile_name"`
	Version     string                `json:"version" yaml:"version"`
	Errors      []validateErrorOutput `json:"errors,omitempty" yaml:"errors,omitempty"`
	Warnings    []string              `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	Stats       validateStatsOutput   `json:"stats" yaml:"stats"`
	Valid       bool                  `json:"valid" yaml:"valid"`
}

type validateErrorOutput struct {
	Type    string `json:"type" yaml:"type"`
	Path    string `json:"path,omitempty" yaml:"path,omitempty"`
	Message string `json:"message" yaml:"message"`
}

type validateStatsOutput struct {
	PluginsUsed  []string `json:"plugins_used,omitempty" yaml:"plugins_used,omitempty"`
	Controls     int      `json:"controls" yaml:"controls"`
	Observations int      `json:"observations" yaml:"observations"`
	Expectations int      `json:"expectations" yaml:"expectations"`
}

func buildValidateOutput(resp *dto.ValidateProfileResponse) validateOutputData {
	out := validateOutputData{
		Valid:       resp.Valid,
		ProfileName: resp.ProfileName,
		Version:     resp.Version,
		Warnings:    resp.Warnings,
		Stats: validateStatsOutput{
			Controls:     resp.Stats.ControlCount,
			Observations: resp.Stats.ObservationCount,
			Expectations: resp.Stats.ExpectCount,
			PluginsUsed:  resp.Stats.PluginsUsed,
		},
	}

	for _, e := range resp.Errors {
		out.Errors = append(out.Errors, validateErrorOutput{
			Type:    e.Type,
			Path:    e.Path,
			Message: e.Message,
		})
	}

	return out
}

func printValidationJSON(resp *dto.ValidateProfileResponse) error {
	out := buildValidateOutput(resp)
	encoder := json.NewEncoder(getStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(out)
}

func printValidationYAML(resp *dto.ValidateProfileResponse) error {
	out := buildValidateOutput(resp)
	encoder := yaml.NewEncoder(getStdout())
	encoder.SetIndent(2)
	return encoder.Encode(out)
}
