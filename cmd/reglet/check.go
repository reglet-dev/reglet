// Package main provides the reglet CLI for compliance and infrastructure validation.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/whiskeyjimbo/reglet/internal/application/dto"
	"github.com/whiskeyjimbo/reglet/internal/domain/execution"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/container"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/output"
)

var (
	format              string
	outFile             string
	trustPlugins        bool
	includeTags         []string
	includeSeverities   []string
	includeControlIDs   []string
	excludeTags         []string
	excludeControlIDs   []string
	filterExpr          string
	includeDependencies bool
)

// checkCmd implements the check command.
var checkCmd = &cobra.Command{
	Use:   "check <profile.yaml>",
	Short: "Execute compliance checks from a profile",
	Long: `Load a profile configuration and execute the defined validation controls.
The profile must be a valid YAML file defining the checks to run.

Filtering:
  Use flags to select specific controls to run.
  --tags security,production    Run controls with 'security' OR 'production' tags
  --severity critical,high      Run controls with 'critical' OR 'high' severity
  --control ssh-check           Run specific controls (exclusive)
  --exclude-tags slow           Exclude controls with 'slow' tag
  --filter "severity == 'high'" Advanced filtering expression
  --include-dependencies        Include dependencies of selected controls`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCheckAction(cmd.Context(), args[0])
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)

	checkCmd.Flags().StringVar(&format, "format", "table", "Output format: table, json, yaml, junit, sarif")
	checkCmd.Flags().StringVarP(&outFile, "output", "o", "", "Output file path (default: stdout)")
	checkCmd.Flags().BoolVar(&trustPlugins, "trust-plugins", false, "Auto-grant all plugin capabilities (use with caution)")

	// Filtering flags
	checkCmd.Flags().StringSliceVar(&includeTags, "tags", nil, "Run controls with these tags (comma-separated)")
	checkCmd.Flags().StringSliceVar(&includeSeverities, "severity", nil, "Run controls with these severities (comma-separated)")
	checkCmd.Flags().StringSliceVar(&includeControlIDs, "control", nil, "Run specific controls by ID (exclusive, comma-separated)")
	checkCmd.Flags().StringSliceVar(&excludeTags, "exclude-tags", nil, "Exclude controls with these tags (comma-separated)")
	checkCmd.Flags().StringSliceVar(&excludeControlIDs, "exclude-control", nil, "Exclude specific controls by ID (comma-separated)")
	checkCmd.Flags().StringVar(&filterExpr, "filter", "", "Advanced filter expression (e.g. \"severity == 'critical'\")")
	checkCmd.Flags().BoolVar(&includeDependencies, "include-dependencies", false, "Include dependencies of selected controls")
}

// runCheckAction encapsulates the logic for the check command.
func runCheckAction(ctx context.Context, profilePath string) error {
	// 1. Create dependency injection container
	c, err := container.New(container.Options{
		TrustPlugins: trustPlugins,
		Logger:       slog.Default(),
	})
	if err != nil {
		return fmt.Errorf("failed to initialize application: %w", err)
	}

	// 2. Build request from CLI flags
	request := buildCheckProfileRequest(profilePath)

	// 3. Execute use case
	response, err := c.CheckProfileUseCase().Execute(ctx, request)
	if err != nil {
		return fmt.Errorf("check failed: %w", err)
	}

	// 4. Format and write output
	if err := writeOutput(response.ExecutionResult, profilePath); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	// 5. Return error if checks failed
	if c.CheckProfileUseCase().CheckFailed(response.ExecutionResult) {
		return fmt.Errorf("check failed: %d passed, %d failed, %d errors",
			response.ExecutionResult.Summary.PassedControls,
			response.ExecutionResult.Summary.FailedControls,
			response.ExecutionResult.Summary.ErrorControls)
	}

	return nil
}

// buildCheckProfileRequest maps CLI flags to a CheckProfileRequest DTO.
func buildCheckProfileRequest(profilePath string) dto.CheckProfileRequest {
	return dto.CheckProfileRequest{
		ProfilePath: profilePath,
		Filters: dto.FilterOptions{
			IncludeTags:         includeTags,
			IncludeSeverities:   includeSeverities,
			IncludeControlIDs:   includeControlIDs,
			ExcludeTags:         excludeTags,
			ExcludeControlIDs:   excludeControlIDs,
			FilterExpression:    filterExpr,
			IncludeDependencies: includeDependencies,
		},
		Options: dto.CheckOptions{
			TrustPlugins: trustPlugins,
		},
		Metadata: dto.RequestMetadata{
			RequestID: generateRequestID(),
		},
	}
}

// writeOutput directs the execution result to the configured output destination.
func writeOutput(result *execution.ExecutionResult, profilePath string) error {
	writer := os.Stdout
	if outFile != "" {
		//nolint:gosec // G304: User-controlled output file path is intentional
		file, err := os.Create(outFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer func() {
			_ = file.Close()
		}()
		writer = file
		slog.Info("writing output", "file", outFile, "format", format)
	}

	return formatOutput(writer, result, format, profilePath)
}

// formatOutput applies the selected formatter to the execution result.
func formatOutput(writer *os.File, result *execution.ExecutionResult, format string, profilePath string) error {
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
	case "junit":
		formatter := output.NewJUnitFormatter(writer)
		return formatter.Format(result)
	case "sarif":
		formatter := output.NewSARIFFormatter(writer, profilePath)
		return formatter.Format(result)
	default:
		return fmt.Errorf("unknown format: %s (supported: table, json, yaml, junit, sarif)", format)
	}
}

// generateRequestID creates a unique identifier for request tracing.
func generateRequestID() string {
	// For now, return empty. In a real implementation, this would generate a UUID
	// or use a correlation ID from the environment (e.g., CI build ID).
	return ""
}
