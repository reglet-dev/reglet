// Package main provides the reglet CLI for compliance and infrastructure validation.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/charmbracelet/huh"
	"github.com/reglet-dev/reglet/internal/infrastructure/scaffold"
	"github.com/spf13/cobra"
)

// initOptions holds the configuration for the init command.
type initOptions struct {
	name       string
	plugins    string // comma-separated
	output     string
	withConfig bool
	force      bool
}

// initCmd is the init command.
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a new Reglet profile with guided wizard",
	Long: `Create a new Reglet profile interactively or with flags.

The init command guides you through creating a starter profile with
example controls for your selected plugins. Optionally generates
a system config file with capability grants for the examples.

Examples:
  # Interactive wizard
  reglet init

  # Non-interactive with flags
  reglet init --name=my-profile --plugins=file,http

  # With custom output path
  reglet init --name=prod --plugins=file --output=./profiles/prod.yaml

  # Generate system config too
  reglet init --name=baseline --plugins=file,http --with-config

  # Overwrite existing files
  reglet init --name=test --plugins=file --force`,
	RunE: runInit,
}

var initOpts initOptions

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringVarP(&initOpts.name, "name", "n", "", "profile name")
	initCmd.Flags().StringVarP(&initOpts.plugins, "plugins", "p", "", "plugins to include (comma-separated: file,http,dns,tcp,command,smtp)")
	initCmd.Flags().StringVarP(&initOpts.output, "output", "o", scaffold.DefaultOutputPath, "output path for generated profile")
	initCmd.Flags().BoolVar(&initOpts.withConfig, "with-config", false, "generate system config with capability grants")
	initCmd.Flags().BoolVarP(&initOpts.force, "force", "f", false, "overwrite existing files without prompting")
}

// runInit is the main entry point for the init command.
func runInit(cmd *cobra.Command, _ []string) error {
	// Set up signal handling for graceful cancellation
	ctx, cancel := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	return runInitWithContext(ctx, cmd)
}

// runInitWithContext runs the init command with the given context.
func runInitWithContext(ctx context.Context, cmd *cobra.Command) error {
	interactive := isInteractive()

	// Build scaffold options, prompting as needed
	opts, err := collectOptions(ctx, cmd, interactive)
	if err != nil {
		if ctx.Err() != nil {
			// User canceled with Ctrl+C - return nil to indicate graceful cancellation
			fmt.Fprintln(os.Stderr, "\nCanceled.")
			return nil //nolint:nilerr // Intentional: Ctrl+C cancellation is not an error
		}
		return err
	}

	// Generate profile (and optionally config)
	gen := scaffold.NewProfileGenerator()
	result, err := gen.Generate(opts)
	if err != nil {
		return fmt.Errorf("generating profile: %w", err)
	}

	// Write files
	if err := writeGeneratedFiles(result, opts, interactive); err != nil {
		return err
	}

	// Print success summary
	printSuccessSummary(result)

	return nil
}

// isInteractive checks if we're running in an interactive terminal.
func isInteractive() bool {
	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// collectOptions gathers all options from flags and interactive prompts.
func collectOptions(ctx context.Context, cmd *cobra.Command, interactive bool) (*scaffold.InitOptions, error) {
	opts := &scaffold.InitOptions{
		ProfileName: initOpts.name,
		OutputPath:  initOpts.output,
		WithConfig:  initOpts.withConfig,
		Force:       initOpts.force,
	}

	// Parse plugins from flag
	if initOpts.plugins != "" {
		opts.Plugins = parsePluginsList(initOpts.plugins)
	}

	// In non-interactive mode, require all flags
	if !interactive {
		return collectNonInteractive(opts)
	}

	// Interactive mode: prompt for missing values
	return collectInteractive(ctx, opts, cmd)
}

// collectNonInteractive validates that all required flags are provided.
func collectNonInteractive(opts *scaffold.InitOptions) (*scaffold.InitOptions, error) {
	if opts.ProfileName == "" {
		return nil, fmt.Errorf("running in non-interactive mode\nRequired flag missing: --name")
	}
	if len(opts.Plugins) == 0 {
		return nil, fmt.Errorf("running in non-interactive mode\nRequired flag missing: --plugins")
	}
	return opts, opts.Validate()
}

// collectInteractive prompts for missing values using huh forms.
func collectInteractive(ctx context.Context, opts *scaffold.InitOptions, cmd *cobra.Command) (*scaffold.InitOptions, error) {
	// Check for context cancellation throughout
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Step 1: Profile name (if not provided via flag)
	if opts.ProfileName == "" {
		name, err := promptProfileName()
		if err != nil {
			return nil, err
		}
		opts.ProfileName = name
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Step 2: Plugin selection (if not provided via flag)
	if len(opts.Plugins) == 0 {
		plugins, err := promptPluginSelection()
		if err != nil {
			return nil, err
		}
		opts.Plugins = plugins
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Step 3: Output path confirmation (can accept default or override)
	if !cmd.Flags().Changed("output") {
		path, err := promptOutputPath(opts.OutputPath)
		if err != nil {
			return nil, err
		}
		opts.OutputPath = path
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Step 4: Config generation (if not specified via flag)
	if !cmd.Flags().Changed("with-config") {
		withConfig, err := promptConfigGeneration()
		if err != nil {
			return nil, err
		}
		opts.WithConfig = withConfig
	}

	return opts, nil
}

// promptProfileName prompts for the profile name.
func promptProfileName() (string, error) {
	var name string

	err := huh.NewInput().
		Title("Profile Name").
		Description("Enter a name for your profile").
		Placeholder("my-profile").
		Validate(scaffold.ValidateProfileName).
		Value(&name).
		Run()

	return name, err
}

// promptPluginSelection prompts for plugin selection.
func promptPluginSelection() ([]string, error) {
	// Build options from available plugins
	options := make([]huh.Option[string], 0, len(scaffold.AvailablePlugins))
	for _, p := range scaffold.AvailablePlugins {
		options = append(options, huh.NewOption(
			fmt.Sprintf("%-9s - %s", p.Name, p.Description),
			p.Name,
		))
	}

	var selected []string

	err := huh.NewMultiSelect[string]().
		Title("Plugin Selection").
		Description("Select plugins to include (space to toggle, enter to confirm)").
		Options(options...).
		Validate(func(s []string) error {
			if len(s) == 0 {
				return fmt.Errorf("select at least one plugin")
			}
			return nil
		}).
		Value(&selected).
		Run()

	return selected, err
}

// promptOutputPath prompts for output path confirmation.
func promptOutputPath(defaultPath string) (string, error) {
	var path string

	err := huh.NewInput().
		Title("Output Path").
		Description(fmt.Sprintf("Profile will be written to: %s", defaultPath)).
		Placeholder(defaultPath).
		Value(&path).
		Run()

	// If empty, use default
	if path == "" {
		path = defaultPath
	}

	return path, err
}

// promptConfigGeneration prompts whether to generate system config.
func promptConfigGeneration() (bool, error) {
	var generate bool

	err := huh.NewConfirm().
		Title("System Configuration").
		Description("Generate ~/.reglet/config.yaml with capability grants?").
		Value(&generate).
		Run()

	return generate, err
}

// parsePluginsList parses a comma-separated list of plugins.
func parsePluginsList(s string) []string {
	parts := strings.Split(s, ",")
	plugins := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			plugins = append(plugins, p)
		}
	}
	return plugins
}

// writeGeneratedFiles writes the profile and optional config files.
func writeGeneratedFiles(result *scaffold.GeneratedProfile, opts *scaffold.InitOptions, interactive bool) error {
	// Check if profile already exists
	if exists, _ := fileExists(result.ProfilePath); exists && !opts.Force {
		if interactive {
			overwrite, err := promptOverwrite(result.ProfilePath)
			if err != nil {
				return err
			}
			if !overwrite {
				fmt.Fprintln(os.Stderr, "Aborted.")
				return nil
			}
		} else {
			return fmt.Errorf("file exists: %s\nUse --force to overwrite", result.ProfilePath)
		}
	}

	// Write profile with atomic write (temp + rename)
	if err := writeFileAtomic(result.ProfilePath, result.ProfileContent); err != nil {
		return fmt.Errorf("writing profile: %w", err)
	}

	// Write config if generated
	if result.ConfigContent != nil {
		configPath, err := scaffold.ExpandConfigPath(result.ConfigPath)
		if err != nil {
			return err
		}

		// Check if config already exists
		if exists, _ := fileExists(configPath); exists && !opts.Force {
			if interactive {
				overwrite, err := promptOverwrite(configPath)
				if err != nil {
					return err
				}
				if !overwrite {
					// Profile was written, but config skipped
					fmt.Fprintf(os.Stderr, "Config generation skipped (file exists).\n")
					return nil
				}
			} else {
				return fmt.Errorf("file exists: %s\nUse --force to overwrite", configPath)
			}
		}

		// Ensure config directory exists
		if err := scaffold.EnsureConfigDir(configPath); err != nil {
			return err
		}

		if err := writeFileAtomic(configPath, result.ConfigContent); err != nil {
			return fmt.Errorf("writing config: %w", err)
		}
	}

	return nil
}

// promptOverwrite prompts whether to overwrite an existing file.
func promptOverwrite(path string) (bool, error) {
	var overwrite bool

	err := huh.NewConfirm().
		Title("File Exists").
		Description(fmt.Sprintf("Overwrite %s?", path)).
		Value(&overwrite).
		Run()

	return overwrite, err
}

// writeFileAtomic writes content to a file atomically using a temp file.
func writeFileAtomic(path string, content []byte) error {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil { //nolint:gosec // 0750 is intentionally permissive for user directories
		return fmt.Errorf("creating directory: %w", err)
	}

	// Write to temp file in same directory
	tempFile, err := os.CreateTemp(dir, ".reglet-init-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tempPath := tempFile.Name()

	// Clean up temp file on error
	defer func() {
		if tempPath != "" {
			_ = os.Remove(tempPath) // Best effort cleanup
		}
	}()

	if _, err := tempFile.Write(content); err != nil {
		_ = tempFile.Close() // Best effort close before returning error
		return fmt.Errorf("writing temp file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("renaming temp file: %w", err)
	}

	// Clear tempPath so cleanup doesn't try to remove it
	tempPath = ""

	return nil
}

// fileExists checks if a file exists.
func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// printSuccessSummary prints the success message with next steps.
func printSuccessSummary(result *scaffold.GeneratedProfile) {
	fmt.Println()
	fmt.Printf("✓ Created profile: %s\n", result.ProfilePath)

	if result.ConfigContent != nil {
		configPath, _ := scaffold.ExpandConfigPath(result.ConfigPath)
		fmt.Printf("✓ Created config:  %s\n", configPath)
	}

	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  reglet check %s\n", result.ProfilePath)
}
