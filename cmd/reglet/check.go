// Package main provides the reglet CLI for compliance and infrastructure validation.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/expr-lang/expr"
	"github.com/spf13/cobra"
	"github.com/whiskeyjimbo/reglet/internal/capabilities"
	"github.com/whiskeyjimbo/reglet/internal/config"
	"github.com/whiskeyjimbo/reglet/internal/engine"
	"github.com/whiskeyjimbo/reglet/internal/output"
	"github.com/whiskeyjimbo/reglet/internal/redaction"
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

// checkCmd represents the check command
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

	checkCmd.Flags().StringVar(&format, "format", "table", "Output format: table, json, yaml, junit")
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

	// Validate profile structure
	if err := config.Validate(profile); err != nil {
		return fmt.Errorf("profile validation failed: %w", err)
	}

	slog.Info("profile validated", "controls", len(profile.Controls.Items))

	// Load system config for redaction and capabilities
	// TODO: Centralize system config loading in root.go or config package
	sysConfig, err := loadSystemConfig()
	if err != nil {
		// Log warning but continue with defaults
		slog.Debug("failed to load system config, using defaults", "error", err)
		sysConfig = &config.SystemConfig{}
	}

	// Initialize Redactor
	redactor, err := redaction.New(redaction.Config{
		Patterns: sysConfig.Redaction.Patterns,
		Paths:    sysConfig.Redaction.Paths,
		HashMode: sysConfig.Redaction.HashMode.Enabled,
		Salt:     sysConfig.Redaction.HashMode.Salt,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize redactor: %w", err)
	}

	// Prepare execution config
	execConfig := engine.DefaultExecutionConfig()
	execConfig.IncludeTags = includeTags
	execConfig.IncludeSeverities = includeSeverities
	execConfig.IncludeControlIDs = includeControlIDs
	execConfig.ExcludeTags = excludeTags
	execConfig.ExcludeControlIDs = excludeControlIDs
	execConfig.IncludeDependencies = includeDependencies

	// Validate filter config
	if err := validateFilterConfig(profile, &execConfig); err != nil {
		return err
	}

	// Determine plugin directory (relative to current working directory or executable)
	pluginDir, err := determinePluginDir()
	if err != nil {
		return fmt.Errorf("failed to determine plugin directory: %w", err)
	}

	// Create capability manager
	capMgr := capabilities.NewManager(trustPlugins)

	// Create execution engine with capability manager and config
	eng, err := engine.NewEngineWithCapabilities(ctx, capMgr, pluginDir, profile, execConfig, redactor)
	if err != nil {
		return fmt.Errorf("failed to create engine: %w", err)
	}
	defer func() {
		_ = eng.Close(ctx) // Best-effort cleanup
	}()

	// Pre-flight schema validation
	// This validates observation configs against plugin schemas BEFORE execution
	slog.Info("validating observation configs against plugin schemas")
	if err := config.ValidateWithSchemas(ctx, profile, eng.Runtime()); err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}
	slog.Info("schema validation complete")

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
		"errors", result.Summary.ErrorControls,
		"skipped", result.Summary.SkippedControls)

	// Determine output writer
	writer := os.Stdout
	if outFile != "" {
		//nolint:gosec // G304: User-controlled output file path is intentional
		file, err := os.Create(outFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer func() {
			_ = file.Close() // Best-effort cleanup
		}()
		writer = file
		slog.Info("writing output", "file", outFile, "format", format)
	}

	// Format and output results
	if err := formatOutput(writer, result, format); err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	// Return non-zero exit code if there were failures or errors
	if result.Summary.FailedControls > 0 || result.Summary.ErrorControls > 0 {
		return fmt.Errorf("check failed: %d passed, %d failed, %d errors",
			result.Summary.PassedControls,
			result.Summary.FailedControls,
			result.Summary.ErrorControls)
	}

	return nil
}

// loadSystemConfig loads the global configuration.
// This duplicates logic from capabilities/manager.go slightly, but we need the struct here.
// Ideally this should be refactored into config package.
func loadSystemConfig() (*config.SystemConfig, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	configPath := filepath.Join(homeDir, ".reglet", "config.yaml")

	return config.LoadSystemConfig(configPath)
}

// validateFilterConfig validates the filter configuration against the profile.
func validateFilterConfig(profile *config.Profile, cfg *engine.ExecutionConfig) error {
	// 1. Validate --control references exist
	if len(cfg.IncludeControlIDs) > 0 {
		controlMap := make(map[string]bool)
		for _, ctrl := range profile.Controls.Items {
			controlMap[ctrl.ID] = true
		}

		for _, id := range cfg.IncludeControlIDs {
			if !controlMap[id] {
				return fmt.Errorf("--control references non-existent control: %s", id)
			}
		}

		// Warn if other filters are specified
		if len(cfg.IncludeTags) > 0 || len(cfg.IncludeSeverities) > 0 || filterExpr != "" {
			fmt.Fprintln(os.Stderr, "⚠️  Warning: --control specified, ignoring other include filters")
		}
	}

	// 2. Compile --filter expression ONCE at startup
	if filterExpr != "" {
		program, err := expr.Compile(filterExpr,
			expr.Env(engine.ControlEnv{}),
			expr.AsBool())
		if err != nil {
			return fmt.Errorf("invalid --filter expression: %w\nExample: severity in ['critical', 'high'] && !('slow' in tags)", err)
		}
		cfg.FilterProgram = program
	}

	// 3. Validate --exclude-control references exist
	if len(cfg.ExcludeControlIDs) > 0 {
		controlMap := make(map[string]bool)
		for _, ctrl := range profile.Controls.Items {
			controlMap[ctrl.ID] = true
		}
		for _, id := range cfg.ExcludeControlIDs {
			if !controlMap[id] {
				return fmt.Errorf("--exclude-control references non-existent control: %s", id)
			}
		}
	}

	return nil
}

// determinePluginDir finds the plugin directory
func determinePluginDir() (string, error) {
	// Try current working directory first
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	pluginDir := filepath.Join(cwd, "plugins")
	if _, err := os.Stat(pluginDir); err == nil {
		return pluginDir, nil
	}

	// Fallback to executable directory
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}

	exeDir := filepath.Dir(exePath)
	pluginDir = filepath.Join(exeDir, "..", "plugins")
	if _, err := os.Stat(pluginDir); err == nil {
		return pluginDir, nil
	}

	return "", fmt.Errorf("plugin directory not found in %s or %s", cwd, exeDir)
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
	case "junit":
		formatter := output.NewJUnitFormatter(writer)
		return formatter.Format(result)
	default:
		return fmt.Errorf("unknown format: %s (supported: table, json, yaml, junit)", format)
	}
}
