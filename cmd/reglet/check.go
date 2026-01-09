// Package main provides the reglet CLI for compliance and infrastructure validation.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/reglet-dev/reglet/internal/application/dto"
	"github.com/reglet-dev/reglet/internal/domain/execution"
	"github.com/reglet-dev/reglet/internal/infrastructure/container"
	"github.com/reglet-dev/reglet/internal/infrastructure/output"
	"github.com/spf13/cobra"
)

// CheckOptions holds the configuration for the check command.
type CheckOptions struct {
	format              string
	outFile             string
	trustPlugins        bool
	securityLevel       string
	includeTags         []string
	includeSeverities   []string
	includeControlIDs   []string
	excludeTags         []string
	excludeControlIDs   []string
	filterExpr          string
	includeDependencies bool
}

func init() {
	rootCmd.AddCommand(newCheckCmd())
}

func newCheckCmd() *cobra.Command {
	opts := &CheckOptions{}

	cmd := &cobra.Command{
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
		Example: `  # Run all controls in a profile
  reglet check profile.yaml

  # Output results as JSON
  reglet check profile.yaml --format json

  # Run only critical and high severity controls
  reglet check profile.yaml --severity critical,high

  # Run controls with security tag, save to file
  reglet check profile.yaml --tags security -o results.json --format json

  # Auto-grant plugin capabilities (CI/CD pipelines)
  reglet check profile.yaml --trust-plugins`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheckAction(cmd.Context(), args[0], opts)
		},
	}

	cmd.Flags().StringVar(&opts.format, "format", "table", "Output format: table, json, yaml, junit, sarif")
	cmd.Flags().StringVarP(&opts.outFile, "output", "o", "", "Output file path (default: stdout)")
	cmd.Flags().BoolVar(&opts.trustPlugins, "trust-plugins", false, "Auto-grant all plugin capabilities (use with caution)")
	cmd.Flags().StringVar(&opts.securityLevel, "security", "", "Security level: strict, standard, permissive (default: standard or config file)")

	// Filtering flags
	cmd.Flags().StringSliceVar(&opts.includeTags, "tags", nil, "Run controls with these tags (comma-separated)")
	cmd.Flags().StringSliceVar(&opts.includeSeverities, "severity", nil, "Run controls with these severities (comma-separated)")
	cmd.Flags().StringSliceVar(&opts.includeControlIDs, "control", nil, "Run specific controls by ID (exclusive, comma-separated)")
	cmd.Flags().StringSliceVar(&opts.excludeTags, "exclude-tags", nil, "Exclude controls with these tags (comma-separated)")
	cmd.Flags().StringSliceVar(&opts.excludeControlIDs, "exclude-control", nil, "Exclude specific controls by ID (comma-separated)")
	cmd.Flags().StringVar(&opts.filterExpr, "filter", "", "Advanced filter expression (e.g. \"severity == 'critical'\")")
	cmd.Flags().BoolVar(&opts.includeDependencies, "include-dependencies", false, "Include dependencies of selected controls")

	return cmd
}

// runCheckAction encapsulates the logic for the check command.
func runCheckAction(ctx context.Context, profilePath string, opts *CheckOptions) error {
	// 1. Create dependency injection container
	// cfgFile comes from the global --config flag in root.go
	c, err := container.New(container.Options{
		TrustPlugins:     opts.trustPlugins,
		SecurityLevel:    opts.securityLevel,
		SystemConfigPath: cfgFile, // Pass config path from CLI flag
		Logger:           slog.Default(),
	})
	if err != nil {
		return fmt.Errorf("failed to initialize application: %w", err)
	}

	// 2. Build request from CLI flags
	request := buildCheckProfileRequest(profilePath, opts)

	// 3. Execute use case
	response, err := c.CheckProfileUseCase().Execute(ctx, request)
	if err != nil {
		return fmt.Errorf("check failed: %w", err)
	}

	// 4. Format and write output
	if err := writeOutput(response.ExecutionResult, profilePath, opts); err != nil {
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
func buildCheckProfileRequest(profilePath string, opts *CheckOptions) dto.CheckProfileRequest {
	return dto.CheckProfileRequest{
		ProfilePath: profilePath,
		Filters: dto.FilterOptions{
			IncludeTags:         opts.includeTags,
			IncludeSeverities:   opts.includeSeverities,
			IncludeControlIDs:   opts.includeControlIDs,
			ExcludeTags:         opts.excludeTags,
			ExcludeControlIDs:   opts.excludeControlIDs,
			FilterExpression:    opts.filterExpr,
			IncludeDependencies: opts.includeDependencies,
		},
		Execution: dto.ExecutionOptions{
			Parallel: true, // Default to parallel execution for performance
			// MaxConcurrentControls and MaxConcurrentObservations will use defaults (0 = auto-detect)
		},
		Options: dto.CheckOptions{
			TrustPlugins: opts.trustPlugins,
		},
		Metadata: dto.RequestMetadata{
			RequestID: generateRequestID(),
		},
	}
}

// writeOutput directs the execution result to the configured output destination.
func writeOutput(result *execution.ExecutionResult, profilePath string, opts *CheckOptions) error {
	writer := os.Stdout
	if opts.outFile != "" {
		//nolint:gosec // G304: User-controlled output file path is intentional
		file, err := os.Create(opts.outFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer func() {
			_ = file.Close()
		}()
		writer = file
		slog.Info("writing output", "file", opts.outFile, "format", opts.format)
	}

	return formatOutput(writer, result, opts.format, profilePath)
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
// Uses cryptographically secure random bytes to ensure uniqueness.
func generateRequestID() string {
	b := make([]byte, 16) // 128 bits of entropy
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails (extremely rare)
		return fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
