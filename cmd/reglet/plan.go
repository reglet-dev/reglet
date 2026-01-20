// Package main provides the reglet CLI for compliance and infrastructure validation.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime"
	"strings"

	"github.com/reglet-dev/reglet/internal/application/dto"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/infrastructure/container"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// PlanOptions holds configuration for the plan command.
type PlanOptions struct {
	filterExpr        string
	includeTags       []string
	includeSeverities []string
	includeControlIDs []string
	excludeTags       []string
	excludeControlIDs []string
	CommonOptions
	showDetails bool
	showTree    bool
}

func init() {
	rootCmd.AddCommand(newPlanCmd())
}

func newPlanCmd() *cobra.Command {
	opts := &PlanOptions{
		CommonOptions: DefaultCommonOptions(),
	}

	cmd := &cobra.Command{
		Use:   "plan <profile.yaml>",
		Short: "Show execution plan without running checks",
		Long: `Generate a dry-run execution plan showing which controls 
would run and in what order, without actually executing them.

The plan shows:
  - Controls grouped by execution level
  - Dependencies between controls  
  - Estimated parallelism

Filtering:
  Use the same filter flags as the check command to see 
  which controls would be selected.`,
		Example: `  # Show execution plan for a profile
  reglet plan profile.yaml

  # Show plan for only critical/high severity controls
  reglet plan profile.yaml --severity critical,high

  # Show plan with detailed output (JSON)
  reglet plan profile.yaml --format json

  # Show detailed control information
  reglet plan profile.yaml --details

  # Show dependency tree
  reglet plan profile.yaml --tree`,
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

			return runPlanAction(cmd.Context(), args[0], opts)
		},
	}

	// Register common flags
	opts.RegisterFlags(cmd)

	// Filtering flags (same as check)
	cmd.Flags().StringSliceVar(&opts.includeTags, "tags", nil,
		"Plan controls with these tags (comma-separated)")
	cmd.Flags().StringSliceVar(&opts.includeSeverities, "severity", nil,
		"Plan controls with these severities (comma-separated)")
	cmd.Flags().StringSliceVar(&opts.includeControlIDs, "control", nil,
		"Plan specific controls by ID (comma-separated)")
	cmd.Flags().StringSliceVar(&opts.excludeTags, "exclude-tags", nil,
		"Exclude controls with these tags (comma-separated)")
	cmd.Flags().StringSliceVar(&opts.excludeControlIDs, "exclude-control", nil,
		"Exclude specific controls by ID (comma-separated)")
	cmd.Flags().StringVar(&opts.filterExpr, "filter", "",
		"Advanced filter expression")

	// Plan-specific flags
	cmd.Flags().BoolVar(&opts.showDetails, "details", false,
		"Show detailed control information")
	cmd.Flags().BoolVar(&opts.showTree, "tree", false,
		"Show dependency tree visualization")

	return cmd
}

func runPlanAction(ctx context.Context, profilePath string, opts *PlanOptions) error {
	// 1. Initialize container
	c, err := container.New(container.Options{
		SystemConfigPath: cfgFile,
		Logger:           slog.Default(),
	})
	if err != nil {
		return fmt.Errorf("failed to initialize application: %w", err)
	}

	// 2. Build request
	request := dto.PlanProfileRequest{
		ProfilePath: profilePath,
		Filters: dto.FilterOptions{
			IncludeTags:       opts.includeTags,
			IncludeSeverities: opts.includeSeverities,
			IncludeControlIDs: opts.includeControlIDs,
			ExcludeTags:       opts.excludeTags,
			ExcludeControlIDs: opts.excludeControlIDs,
			FilterExpression:  opts.filterExpr,
		},
		Metadata: dto.RequestMetadata{
			RequestID: generateRequestID(),
		},
	}

	// 3. Execute
	response, err := c.PlanProfileUseCase().Execute(ctx, request)
	if err != nil {
		return fmt.Errorf("plan failed: %w", err)
	}

	// 4. Output plan
	return printPlan(response.Plan, opts)
}

func printPlan(plan *entities.ExecutionPlan, opts *PlanOptions) error {
	// Handle JSON/YAML output
	switch opts.Format {
	case "json":
		return printPlanJSON(plan)
	case "yaml":
		return printPlanYAML(plan)
	default:
		// Check for tree view
		if opts.showTree {
			return printPlanTree(plan)
		}
		return printPlanTable(plan, opts)
	}
}

func printPlanTable(plan *entities.ExecutionPlan, opts *PlanOptions) error {
	fmt.Printf("\nExecution Plan for: %s v%s\n", plan.ProfileName, plan.ProfileVersion)
	fmt.Println(strings.Repeat("─", 60))

	if plan.IsEmpty() {
		fmt.Println("\n  No controls match the current filter criteria.")
		return nil
	}

	for _, level := range plan.Levels {
		if len(level.Controls) == 1 {
			fmt.Printf("\nLevel %d (sequential):\n", level.Level)
		} else {
			fmt.Printf("\nLevel %d (parallel - %d controls):\n", level.Level, len(level.Controls))
		}
		for _, ctrl := range level.Controls {
			printControlSummary(ctrl, opts.showDetails)
		}
	}

	// Summary section
	fmt.Println()
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("Summary:")
	fmt.Printf("  Total Controls:    %d\n", plan.TotalControls)
	fmt.Printf("  Execution Levels:  %d\n", plan.LevelCount())
	fmt.Printf("  Max Parallelism:   %d (of %d CPUs)\n", plan.MaxParallelism, runtime.NumCPU())

	if plan.HasDependencies {
		fmt.Println("  Dependencies:      Yes (controls will run in dependency order)")
	} else {
		fmt.Println("  Dependencies:      None (all controls can run in parallel)")
	}

	fmt.Println()
	return nil
}

// printPlanTree displays the execution plan as an ASCII DAG flowchart.
// Shows controls at each level with arrows indicating dependency flow.
func printPlanTree(plan *entities.ExecutionPlan) error {
	fmt.Printf("\nExecution Flow for: %s v%s\n", plan.ProfileName, plan.ProfileVersion)
	fmt.Println(strings.Repeat("─", 70))

	if plan.IsEmpty() {
		fmt.Println("\n  No controls match the current filter criteria.")
		return nil
	}

	// Build lookup map
	allControls := make(map[string]entities.ControlSummary)
	for _, level := range plan.Levels {
		for _, ctrl := range level.Controls {
			allControls[ctrl.ID] = ctrl
		}
	}

	// Print each level
	for i, level := range plan.Levels {
		// Level header
		fmt.Printf("\n┌─ Level %d ", level.Level)
		if len(level.Controls) == 1 {
			fmt.Println("(sequential) ─────────────────────────────────────────────")
		} else {
			fmt.Printf("(parallel × %d) ", len(level.Controls))
			fmt.Println(strings.Repeat("─", 50-len(fmt.Sprintf("%d", len(level.Controls)))))
		}
		fmt.Println("│")

		// Sort controls for consistent output
		var ctrlIDs []string
		for _, ctrl := range level.Controls {
			ctrlIDs = append(ctrlIDs, ctrl.ID)
		}
		sortStrings(ctrlIDs)

		// Print controls at this level
		for _, id := range ctrlIDs {
			ctrl := allControls[id]
			severity := ""
			if ctrl.Severity != "" {
				severity = fmt.Sprintf(" [%s]", ctrl.Severity)
			}

			// Show what this control depends on
			if len(ctrl.DependsOn) > 0 {
				deps := strings.Join(ctrl.DependsOn, ", ")
				fmt.Printf("│  ◆ %s%s\n", id, severity)
				fmt.Printf("│    └─ depends on: %s\n", deps)
			} else {
				fmt.Printf("│  ○ %s%s\n", id, severity)
			}
		}

		// Show flow arrow between levels
		if i < len(plan.Levels)-1 {
			fmt.Println("│")
			fmt.Println("▼")
		}
	}

	// Summary
	fmt.Println()
	fmt.Println(strings.Repeat("─", 70))

	// Count roots
	rootCount := 0
	for _, ctrl := range allControls {
		if len(ctrl.DependsOn) == 0 {
			rootCount++
		}
	}

	fmt.Printf("Legend: ○ = root (no deps)  ◆ = has dependencies  ▼ = execution flow\n")
	fmt.Printf("Total: %d controls, %d root(s), %d level(s)\n\n",
		plan.TotalControls, rootCount, plan.LevelCount())

	return nil
}

// sortStrings sorts a slice of strings in place.
func sortStrings(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

func printControlSummary(ctrl entities.ControlSummary, showDetails bool) {
	// Base output: checkmark, ID, and name
	deps := ""
	if len(ctrl.DependsOn) > 0 {
		deps = fmt.Sprintf(" (depends on: %s)", strings.Join(ctrl.DependsOn, ", "))
	}

	if showDetails {
		// Detailed format with severity and tags
		severity := ""
		if ctrl.Severity != "" {
			severity = fmt.Sprintf(" [%s]", ctrl.Severity)
		}
		tags := ""
		if len(ctrl.Tags) > 0 {
			tags = fmt.Sprintf(" {%s}", strings.Join(ctrl.Tags, ", "))
		}
		// Always show observation and expectation counts for consistency
		counts := fmt.Sprintf(" (%d obs, %d expect)", ctrl.Observations, ctrl.Expectations)
		fmt.Printf("  ✓ %s: %s%s%s%s%s\n", ctrl.ID, ctrl.Name, severity, tags, counts, deps)
	} else {
		// Simple format
		fmt.Printf("  ✓ %s: %s%s\n", ctrl.ID, ctrl.Name, deps)
	}
}

// planOutputData is the structured output format for JSON/YAML.
type planOutputData struct {
	Profile struct {
		Name    string   `json:"name" yaml:"name"`
		Version string   `json:"version" yaml:"version"`
		Source  string   `json:"source,omitempty" yaml:"source,omitempty"`
		Plugins []string `json:"plugins,omitempty" yaml:"plugins,omitempty"`
	} `json:"profile" yaml:"profile"`
	Levels  []planLevelOutput `json:"levels" yaml:"levels"`
	Summary struct {
		TotalControls   int  `json:"total_controls" yaml:"total_controls"`
		ExecutionLevels int  `json:"execution_levels" yaml:"execution_levels"`
		MaxParallelism  int  `json:"max_parallelism" yaml:"max_parallelism"`
		CPUCount        int  `json:"cpu_count" yaml:"cpu_count"`
		HasDependencies bool `json:"has_dependencies" yaml:"has_dependencies"`
	} `json:"summary" yaml:"summary"`
}

type planLevelOutput struct {
	Controls []planControlOutput `json:"controls" yaml:"controls"`
	Level    int                 `json:"level" yaml:"level"`
}

type planControlOutput struct {
	ID           string   `json:"id" yaml:"id"`
	Name         string   `json:"name" yaml:"name"`
	Severity     string   `json:"severity,omitempty" yaml:"severity,omitempty"`
	Tags         []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	DependsOn    []string `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	Observations int      `json:"observations" yaml:"observations"`
	Expectations int      `json:"expectations" yaml:"expectations"`
}

func buildPlanOutput(plan *entities.ExecutionPlan) planOutputData {
	out := planOutputData{}
	out.Profile.Name = plan.ProfileName
	out.Profile.Version = plan.ProfileVersion

	for _, level := range plan.Levels {
		levelOut := planLevelOutput{Level: level.Level}
		for _, ctrl := range level.Controls {
			levelOut.Controls = append(levelOut.Controls, planControlOutput{
				ID:           ctrl.ID,
				Name:         ctrl.Name,
				Severity:     ctrl.Severity,
				Tags:         ctrl.Tags,
				DependsOn:    ctrl.DependsOn,
				Observations: ctrl.Observations,
				Expectations: ctrl.Expectations,
			})
		}
		out.Levels = append(out.Levels, levelOut)
	}

	out.Summary.TotalControls = plan.TotalControls
	out.Summary.ExecutionLevels = plan.LevelCount()
	out.Summary.MaxParallelism = plan.MaxParallelism
	out.Summary.CPUCount = runtime.NumCPU()
	out.Summary.HasDependencies = plan.HasDependencies

	return out
}

func printPlanJSON(plan *entities.ExecutionPlan) error {
	out := buildPlanOutput(plan)
	encoder := json.NewEncoder(getStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(out)
}

func printPlanYAML(plan *entities.ExecutionPlan) error {
	out := buildPlanOutput(plan)
	encoder := yaml.NewEncoder(getStdout())
	encoder.SetIndent(2)
	return encoder.Encode(out)
}

// getStdout returns os.Stdout for output. This is a function to allow testing.
func getStdout() *writerAdapter {
	return &writerAdapter{}
}

// writerAdapter wraps os.Stdout for use with encoders.
type writerAdapter struct{}

func (w *writerAdapter) Write(p []byte) (n int, err error) {
	fmt.Print(string(p))
	return len(p), nil
}
