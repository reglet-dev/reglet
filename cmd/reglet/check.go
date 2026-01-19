// Package main provides the reglet CLI for compliance and infrastructure validation.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/reglet-dev/reglet/internal/application/dto"
	"github.com/reglet-dev/reglet/internal/application/ports"
	"github.com/reglet-dev/reglet/internal/domain/execution"
	"github.com/reglet-dev/reglet/internal/infrastructure/config"
	"github.com/reglet-dev/reglet/internal/infrastructure/container"
	"github.com/reglet-dev/reglet/internal/infrastructure/output"
	"github.com/reglet-dev/reglet/internal/infrastructure/watcher"
	"github.com/spf13/cobra"
)

// CheckOptions holds the configuration for the check command.
type CheckOptions struct {
	securityLevel     string
	filterExpr        string
	outFile           string
	setFileVars       []string
	includeTags       []string
	includeSeverities []string
	includeControlIDs []string
	excludeTags       []string
	excludeControlIDs []string
	setEnvVars        []string
	setVars           []string
	CommonOptions
	watchInterval       time.Duration
	fetchTimeout        time.Duration
	watch               bool
	showDetails         bool
	includeDependencies bool
	trustPlugins        bool
	noWarnUnusedVars    bool
	allowPrivateNetwork bool
	refresh             bool
	insecure            bool
}

func init() {
	rootCmd.AddCommand(newCheckCmd())
}

func newCheckCmd() *cobra.Command {
	opts := &CheckOptions{
		CommonOptions: DefaultCommonOptions(),
	}

	cmd := &cobra.Command{
		Use:   "check <profile.yaml | https://...>",
		Short: "Execute compliance checks from a profile",
		Long: `Load a profile configuration and execute the defined validation controls.
The profile can be a local YAML file or a remote URL (HTTPS or OCI).

Remote Profiles:
  Run checks directly from a URL:
  reglet check https://example.com/profiles/security.yaml
  reglet check oci://ghcr.io/org/profiles/baseline:v1.0

  Remote profiles are cached locally for 1 hour by default.
  Use --refresh to bypass cache and force re-fetch.

Filtering:
  Use flags to select specific controls to run.
  --tags security,production    Run controls with 'security' OR 'production' tags
  --severity critical,high      Run controls with 'critical' OR 'high' severity
  --control ssh-check           Run specific controls (exclusive)
  --exclude-tags slow           Exclude controls with 'slow' tag
  --filter "severity == 'high'" Advanced filtering expression
  --include-dependencies        Include dependencies of selected controls

Variable Overrides:
  Override or inject profile variables from the command line.
  --set environment=prod        Override {{ .vars.environment }} with "prod"
  --set paths.config=/custom    Override nested variable {{ .vars.paths.config }}
  --set port=8080               Auto-detect type: integers, floats, booleans

Watch Mode:
  Use --watch to continuously monitor files and re-run checks on changes.
  --watch                       Enable watch mode
  --interval=2s                 Debounce interval (default: 2s)`,
		Example: `  # Run all controls in a profile
  reglet check profile.yaml

  # Output results as JSON
  reglet check profile.yaml --format json

  # Run only critical and high severity controls
  reglet check profile.yaml --severity critical,high

  # Run controls with security tag, save to file
  reglet check profile.yaml --tags security -o results.json --format json

  # Override profile variables from CLI
  reglet check profile.yaml --set environment=prod --set debug=true

  # Override nested variables
  reglet check profile.yaml --set paths.config=/opt/custom

  # Auto-grant plugin capabilities (CI/CD pipelines)
  reglet check profile.yaml --trust-plugins

  # Watch mode: re-run checks when files change
  reglet check profile.yaml --watch

  # Watch mode with custom debounce interval
  reglet check profile.yaml --watch --interval=500ms

  # Run checks from a remote profile
  reglet check https://example.com/profiles/security.yaml

  # Force re-fetch of cached remote profile
  reglet check https://example.com/profiles/security.yaml --refresh

  # Allow fetching from internal network (use with caution)
  reglet check https://internal.corp/profiles/test.yaml --allow-private-network`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate common flags
			if err := opts.ValidateFlags(); err != nil {
				return err
			}

			// Apply logging overrides
			if opts.Quiet {
				quiet = true
				setupLogging()
			} else if opts.Verbose {
				logLevel = "debug"
				setupLogging()
			}

			// Route to watch mode or single-run mode
			if opts.watch {
				return runWatchMode(cmd.Context(), args[0], opts)
			}
			return runCheckAction(cmd.Context(), args[0], opts)
		},
	}

	// Register common flags
	opts.RegisterFlags(cmd)

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
	cmd.Flags().BoolVar(&opts.showDetails, "details", false, "Show detailed evidence for loop observations")

	// Watch mode flags
	cmd.Flags().BoolVar(&opts.watch, "watch", false, "Enable watch mode: re-run checks when files change")
	cmd.Flags().DurationVar(&opts.watchInterval, "interval", 2*time.Second, "Debounce interval for watch mode (e.g., 500ms, 2s, 1m)")

	// Variable override flags
	cmd.Flags().StringSliceVar(&opts.setVars, "set", nil, "Set variable values (key=value, can be repeated)")
	cmd.Flags().StringSliceVar(&opts.setFileVars, "set-file", nil, "Set variable from file (key=path, can be repeated)")
	cmd.Flags().StringSliceVar(&opts.setEnvVars, "set-env", nil, "Set variable from environment (key=ENV_VAR, can be repeated)")
	cmd.Flags().BoolVar(&opts.noWarnUnusedVars, "no-warn-unused-vars", false, "Suppress warnings about unused CLI variables")

	// Remote profile flags
	cmd.Flags().BoolVar(&opts.allowPrivateNetwork, "allow-private-network", false, "Allow fetching profiles from private IP addresses (SSRF bypass)")
	cmd.Flags().DurationVar(&opts.fetchTimeout, "fetch-timeout", 30*time.Second, "Timeout for remote profile fetching")
	cmd.Flags().BoolVar(&opts.refresh, "refresh", false, "Bypass cache and force re-fetch of remote profile")
	cmd.Flags().BoolVar(&opts.insecure, "insecure", false, "Skip TLS certificate verification for remote profiles (not recommended)")

	return cmd
}

// runCheckAction encapsulates the logic for the check command.
func runCheckAction(ctx context.Context, profilePath string, opts *CheckOptions) error {
	// 1. Initialize container (uses global cfgFile)
	c, err := container.New(container.Options{
		TrustPlugins:     opts.trustPlugins,
		SecurityLevel:    opts.securityLevel,
		SystemConfigPath: cfgFile, // Pass config path from CLI flag
		Logger:           slog.Default(),
	})
	if err != nil {
		return fmt.Errorf("failed to initialize application: %w", err)
	}

	// 2. Build request
	request, err := buildCheckProfileRequest(profilePath, opts)
	if err != nil {
		return err
	}

	// 2b. Emit unused var warnings (if enabled and CLI vars provided)
	if !opts.noWarnUnusedVars && len(request.CLIVariables) > 0 {
		emitUnusedVarWarnings(profilePath, request.CLIVariables)
	}

	// 3. Apply timeout to context
	ctx, cancel := opts.ApplyToContext(ctx)
	defer cancel()

	// 4. Execute
	response, err := c.CheckProfileUseCase().Execute(ctx, request)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("execution exceeded global timeout (%s)", opts.Timeout)
		}
		return fmt.Errorf("check failed: %w", err)
	}

	// 5. Write output
	if err := writeOutput(c.OutputFormatterFactory(), response.ExecutionResult, profilePath, opts); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	// 6. Verify results
	if c.CheckProfileUseCase().CheckFailed(response.ExecutionResult) {
		return fmt.Errorf("check failed: %d passed, %d failed, %d errors",
			response.ExecutionResult.Summary.PassedControls,
			response.ExecutionResult.Summary.FailedControls,
			response.ExecutionResult.Summary.ErrorControls)
	}

	return nil
}

// emitUnusedVarWarnings reads the profile and warns about CLI vars that aren't referenced.
func emitUnusedVarWarnings(profilePath string, cliVars map[string]interface{}) {
	// Read profile content
	content, err := os.ReadFile(filepath.Clean(profilePath))
	if err != nil {
		// Silently skip - profile load errors will be caught later
		return
	}

	unused := config.FindUnusedVars(cliVars, string(content))
	for _, key := range unused {
		slog.Warn("CLI variable not referenced in profile", "variable", key)
	}
}

// buildCheckProfileRequest maps CLI flags to a CheckProfileRequest DTO.
func buildCheckProfileRequest(profilePath string, opts *CheckOptions) (dto.CheckProfileRequest, error) {
	// Initialize result map for all CLI vars
	cliVars := make(map[string]interface{})

	// Parse --set flags
	if len(opts.setVars) > 0 {
		parsed, err := config.ParseMultipleCLIVars(opts.setVars)
		if err != nil {
			return dto.CheckProfileRequest{}, fmt.Errorf("invalid --set value: %w", err)
		}
		for k, v := range parsed {
			cliVars[k] = v
		}
	}

	// Parse --set-file flags (file content wins over --set for same key)
	for _, input := range opts.setFileVars {
		key, value, err := config.ParseSetFile(input)
		if err != nil {
			return dto.CheckProfileRequest{}, fmt.Errorf("invalid --set-file value: %w", err)
		}
		if err := config.SetNestedValue(cliVars, key, value); err != nil {
			return dto.CheckProfileRequest{}, fmt.Errorf("setting --set-file %q: %w", key, err)
		}
	}

	// Parse --set-env flags (env wins over --set and --set-file for same key)
	for _, input := range opts.setEnvVars {
		key, value, err := config.ParseSetEnv(input)
		if err != nil {
			return dto.CheckProfileRequest{}, fmt.Errorf("invalid --set-env value: %w", err)
		}
		if err := config.SetNestedValue(cliVars, key, value); err != nil {
			return dto.CheckProfileRequest{}, fmt.Errorf("setting --set-env %q: %w", key, err)
		}
	}

	// Set nil if empty to match existing behavior
	if len(cliVars) == 0 {
		cliVars = nil
	}

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
			Parallel: opts.Parallel, // Use common option
			// MaxConcurrentControls and MaxConcurrentObservations will use defaults (0 = auto-detect)
		},
		Options: dto.CheckOptions{
			TrustPlugins:   opts.trustPlugins,
			WarnUnusedVars: true, // Default to warning about unused vars
		},
		Metadata: dto.RequestMetadata{
			RequestID: generateRequestID(),
		},
		CLIVariables: cliVars,
	}, nil
}

// writeOutput directs the execution result to the configured output destination.
func writeOutput(factory ports.OutputFormatterFactory, result *execution.ExecutionResult, profilePath string, opts *CheckOptions) error {
	var writer io.Writer = os.Stdout
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
		slog.Info("writing output", "file", opts.outFile, "format", opts.Format)
	}

	return formatOutput(factory, writer, result, opts, profilePath)
}

// formatOutput applies the selected formatter to the execution result.
func formatOutput(factory ports.OutputFormatterFactory, writer io.Writer, result *execution.ExecutionResult, opts *CheckOptions, profilePath string) error {
	formatter, err := factory.Create(
		opts.Format,
		writer,
		output.FactoryOptions{
			Indent:      true,
			ProfilePath: profilePath,
			ShowDetails: opts.showDetails,
		},
	)
	if err != nil {
		return err
	}
	return formatter.Format(result)
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

// runWatchMode implements the watch loop for continuous compliance monitoring.
// It watches the profile file and any referenced files for changes, re-running
// checks with a configurable debounce interval.
func runWatchMode(ctx context.Context, profilePath string, opts *CheckOptions) error {
	// Validate and setup
	ws, err := newWatchSession(profilePath, opts)
	if err != nil {
		return err
	}
	defer func() { _ = ws.watcher.Close() }()

	// Set up signal handling for graceful shutdown
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ws.printStatus(fmt.Sprintf("Watching %s for changes...", profilePath))
	ws.printStatus("Running initial check...")
	fmt.Println()

	// Run initial check
	if err := runCheckAction(ctx, profilePath, opts); err != nil {
		if ctx.Err() != nil {
			return nil // User canceled
		}
		slog.Warn("initial check failed", "error", err)
	}
	ws.checksExecuted++

	return ws.runLoop(ctx, profilePath, opts)
}

// watchSession holds state for a watch mode session.
type watchSession struct {
	startTime      time.Time
	watcher        *watcher.FSNotifyWatcher
	absPath        string
	watchInterval  time.Duration
	checksExecuted int
}

// newWatchSession validates options and creates a watch session.
func newWatchSession(profilePath string, opts *CheckOptions) (*watchSession, error) {
	if opts.watchInterval <= 0 {
		return nil, fmt.Errorf("invalid interval value %q: duration must be positive", opts.watchInterval)
	}
	if opts.watchInterval > time.Hour {
		return nil, fmt.Errorf("invalid interval value %q: duration must not exceed 1 hour", opts.watchInterval)
	}

	absPath, err := filepath.Abs(profilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve profile path: %w", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("profile not found: %s", absPath)
	}

	w, err := watcher.NewFSNotifyWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize file watcher: %w", err)
	}

	profileDir := filepath.Dir(absPath)
	if err := w.Add(profileDir); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("failed to watch profile directory: %w", err)
	}

	return &watchSession{
		watcher:       w,
		absPath:       absPath,
		startTime:     time.Now(),
		watchInterval: opts.watchInterval,
	}, nil
}

// printStatus outputs a timestamped status message.
func (ws *watchSession) printStatus(msg string) {
	fmt.Printf("[%s] %s\n", time.Now().Format("15:04:05"), msg)
}

// printSummary outputs the session summary.
func (ws *watchSession) printSummary() {
	fmt.Println()
	ws.printStatus(fmt.Sprintf("Watch mode stopped. Executed %d checks in %s.",
		ws.checksExecuted, time.Since(ws.startTime).Round(time.Second)))
}

// runLoop executes the main watch loop.
func (ws *watchSession) runLoop(ctx context.Context, profilePath string, opts *CheckOptions) error {
	var debounceTimer *time.Timer
	debounceChan := make(chan struct{}, 1)

	for {
		select {
		case <-ctx.Done():
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			ws.printSummary()
			return nil

		case event := <-ws.watcher.Events():
			if !ws.isRelevantEvent(event) {
				continue
			}
			ws.printStatus(fmt.Sprintf("Change detected: %s", filepath.Base(event.Path)))
			debounceTimer = ws.resetDebounce(debounceTimer, debounceChan)

		case err := <-ws.watcher.Errors():
			slog.Error("watcher error", "error", err)

		case <-debounceChan:
			fmt.Println()
			ws.printStatus("Running check...")
			fmt.Println()

			if err := runCheckAction(ctx, profilePath, opts); err != nil {
				if ctx.Err() != nil {
					ws.printSummary()
					return nil //nolint:nilerr // Intentionally return nil on user cancellation
				}
				slog.Warn("check failed", "error", err)
			}
			ws.checksExecuted++
		}
	}
}

// isRelevantEvent checks if the event is for the watched profile.
func (ws *watchSession) isRelevantEvent(event watcher.Event) bool {
	if event.Path != ws.absPath && filepath.Base(event.Path) != filepath.Base(ws.absPath) {
		return false
	}
	return event.Op&(watcher.Write|watcher.Create) != 0
}

// resetDebounce stops the old timer and starts a new one.
func (ws *watchSession) resetDebounce(timer *time.Timer, ch chan struct{}) *time.Timer {
	if timer != nil {
		timer.Stop()
	}
	return time.AfterFunc(ws.watchInterval, func() {
		select {
		case ch <- struct{}{}:
		default:
		}
	})
}
