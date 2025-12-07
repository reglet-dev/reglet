package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/whiskeyjimbo/reglet/internal/config"
	"github.com/whiskeyjimbo/reglet/internal/redaction"
	"github.com/whiskeyjimbo/reglet/internal/wasm"
	"github.com/whiskeyjimbo/reglet/internal/wasm/hostfuncs"
	"golang.org/x/sync/errgroup"
)

// ExecutionConfig controls execution behavior.
type ExecutionConfig struct {
	// MaxConcurrentControls limits parallel control execution (0 = no limit)
	MaxConcurrentControls int
	// MaxConcurrentObservations limits parallel observation execution within a control (0 = no limit)
	MaxConcurrentObservations int
	// Parallel enables parallel execution (default: true for performance)
	Parallel bool

	// Include Filters (OR logic within slice, AND between types)
	IncludeTags       []string
	IncludeSeverities []string
	IncludeControlIDs []string // Exclusive - if set, other filters ignored

	// Exclude Filters (take precedence over includes)
	ExcludeTags       []string
	ExcludeControlIDs []string

	// Advanced Filter (Compiled Expression)
	FilterProgram *vm.Program

	// Dependency Strategy
	IncludeDependencies bool
}

// ControlEnv exposes control metadata for expression evaluation.
type ControlEnv struct {
	ID       string   `expr:"id"`
	Name     string   `expr:"name"`
	Severity string   `expr:"severity"`
	Owner    string   `expr:"owner"`
	Tags     []string `expr:"tags"`
}

// DefaultExecutionConfig returns sensible defaults for parallel execution.
func DefaultExecutionConfig() ExecutionConfig {
	return ExecutionConfig{
		MaxConcurrentControls:     10, // Reasonable default
		MaxConcurrentObservations: 5,  // Conservative to avoid overwhelming systems
		Parallel:                  true,
	}
}

// Engine coordinates profile execution.
type Engine struct {
	runtime  *wasm.Runtime
	executor *ObservationExecutor
	config   ExecutionConfig
}

// CapabilityManager defines the interface for capability management
type CapabilityManager interface {
	CollectRequiredCapabilities(ctx context.Context, profile *config.Profile, runtime *wasm.Runtime, pluginDir string) (map[string][]hostfuncs.Capability, error)
	GrantCapabilities(required map[string][]hostfuncs.Capability) (map[string][]hostfuncs.Capability, error)
}

// NewEngine creates a new execution engine with default configuration.
func NewEngine(ctx context.Context) (*Engine, error) {
	return NewEngineWithConfig(ctx, DefaultExecutionConfig())
}

// NewEngineWithCapabilities creates an engine with interactive capability prompts
func NewEngineWithCapabilities(ctx context.Context, capMgr CapabilityManager, pluginDir string, profile *config.Profile, cfg ExecutionConfig, redactor *redaction.Redactor) (*Engine, error) {
	// Create temporary runtime with no capabilities to load plugins and get requirements
	tempRuntime, err := wasm.NewRuntime(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary runtime: %w", err)
	}

	// Collect required capabilities from all plugins
	required, err := capMgr.CollectRequiredCapabilities(ctx, profile, tempRuntime, pluginDir)
	if err != nil {
		_ = tempRuntime.Close(ctx) // Best-effort cleanup
		return nil, fmt.Errorf("failed to collect capabilities: %w", err)
	}

	// Close temporary runtime
	_ = tempRuntime.Close(ctx) // Best-effort cleanup

	// Get granted capabilities (will prompt user if needed)
	granted, err := capMgr.GrantCapabilities(required)
	if err != nil {
		return nil, fmt.Errorf("failed to grant capabilities: %w", err)
	}

	// Create WASM runtime with granted capabilities
	runtime, err := wasm.NewRuntimeWithCapabilities(ctx, granted)
	if err != nil {
		return nil, fmt.Errorf("failed to create WASM runtime: %w", err)
	}

	// Create observation executor
	executor := NewExecutor(runtime, pluginDir, redactor)

	// Preload plugins for schema validation
	for _, ctrl := range profile.Controls.Items {
		for _, obs := range ctrl.Observations {
			if _, err := executor.LoadPlugin(ctx, obs.Plugin); err != nil {
				return nil, fmt.Errorf("failed to preload plugin %s: %w", obs.Plugin, err)
			}
		}
	}

	return &Engine{
		runtime:  runtime,
		executor: executor,
		config:   cfg,
	}, nil
}

// NewEngineWithConfig creates a new execution engine with custom configuration.
func NewEngineWithConfig(ctx context.Context, cfg ExecutionConfig) (*Engine, error) {
	// Create WASM runtime with no capabilities (legacy path)
	runtime, err := wasm.NewRuntime(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create WASM runtime: %w", err)
	}

	// Create observation executor with no redactor
	executor := NewObservationExecutor(runtime, nil)

	return &Engine{
		runtime:  runtime,
		executor: executor,
		config:   cfg,
	}, nil
}

// Execute runs a complete profile and returns the result.
func (e *Engine) Execute(ctx context.Context, profile *config.Profile) (*ExecutionResult, error) {
	// Create execution result
	result := NewExecutionResult(profile.Metadata.Name, profile.Metadata.Version)

	// Calculate required dependencies if enabled
	var requiredControls map[string]bool
	if e.config.IncludeDependencies {
		requiredControls = e.resolveDependencies(profile)
	}

	// Execute controls
	if e.config.Parallel && len(profile.Controls.Items) > 1 {
		// Parallel execution of controls
		if err := e.executeControlsParallel(ctx, profile.Controls.Items, result, requiredControls); err != nil {
			return nil, err
		}
	} else {
		// Sequential execution of controls
		for _, ctrl := range profile.Controls.Items {
			controlResult := e.executeControl(ctx, ctrl, result, requiredControls)
			result.AddControlResult(controlResult)
		}
	}

	// Finalize result (calculate summary, set end time)
	result.Finalize()

	return result, nil
}

// resolveDependencies calculates the transitive closure of dependencies for matched controls.
func (e *Engine) resolveDependencies(profile *config.Profile) map[string]bool {
	required := make(map[string]bool)
	queue := make([]string, 0)
	controlMap := make(map[string]config.Control)

	// Index controls for fast lookup
	for _, ctrl := range profile.Controls.Items {
		controlMap[ctrl.ID] = ctrl
	}

	// Identify initial targets (controls that match filters)
	for _, ctrl := range profile.Controls.Items {
		if should, _ := e.shouldRun(ctrl); should {
			// Add dependencies to queue
			queue = append(queue, ctrl.DependsOn...)
		}
	}

	// Process queue to find all transitive dependencies
	visited := make(map[string]bool)
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]

		if visited[id] {
			continue
		}
		visited[id] = true

		// If dependency exists in profile, mark as required and add its dependencies
		if ctrl, exists := controlMap[id]; exists {
			required[id] = true
			queue = append(queue, ctrl.DependsOn...)
		}
	}

	return required
}

// executeControlsParallel executes controls in parallel, respecting dependencies.
// Controls are organized into levels by BuildControlDAG, and each level is executed
// sequentially while controls within a level run in parallel.
func (e *Engine) executeControlsParallel(ctx context.Context, controls []config.Control, result *ExecutionResult, requiredDeps map[string]bool) error {
	// Build dependency graph and get control levels
	levels, err := BuildControlDAG(controls)
	if err != nil {
		return fmt.Errorf("failed to build control dependency graph: %w", err)
	}

	// Execute each level sequentially, controls within level in parallel
	for _, level := range levels {
		// Create errgroup for this level
		g, levelCtx := errgroup.WithContext(ctx)

		// Apply concurrency limit if specified
		if e.config.MaxConcurrentControls > 0 {
			g.SetLimit(e.config.MaxConcurrentControls)
		}

		// Execute all controls in this level in parallel
		for _, ctrl := range level.Controls {
			g.Go(func() error {
				controlResult := e.executeControl(levelCtx, ctrl, result, requiredDeps)
				result.AddControlResult(controlResult)
				return nil // Don't fail fast on individual control errors
			})
		}

		// Wait for all controls in this level to complete before moving to next level
		if err := g.Wait(); err != nil {
			return fmt.Errorf("level %d execution failed: %w", level.Level, err)
		}
	}

	return nil
}

// executeControl executes a single control and returns its result.
func (e *Engine) executeControl(ctx context.Context, ctrl config.Control, execResult *ExecutionResult, requiredDeps map[string]bool) ControlResult {
	startTime := time.Now()

	result := ControlResult{
		ID:           ctrl.ID,
		Name:         ctrl.Name,
		Description:  ctrl.Description,
		Severity:     ctrl.Severity,
		Tags:         ctrl.Tags,
		Observations: make([]ObservationResult, 0, len(ctrl.Observations)),
	}

	// Check if control should run (filtering)
	shouldRun, skipReason := e.shouldRun(ctrl)
	
	// If filtering says skip, check if it's required as a dependency
	if !shouldRun && e.config.IncludeDependencies && requiredDeps[ctrl.ID] {
		shouldRun = true
		skipReason = "" // Clear skip reason as we are running it
	}

	if !shouldRun {
		result.Status = StatusSkipped
		result.SkipReason = skipReason
		result.Message = skipReason
		result.Duration = time.Since(startTime)
		return result
	}

	// Check dependencies before execution
	if len(ctrl.DependsOn) > 0 {
		// Look up dependency statuses from previous results
		for _, depID := range ctrl.DependsOn {
			depStatus, found := execResult.GetControlStatus(depID)

			// If dependency not found or failed/error, skip this control
			if !found || depStatus == StatusFail || depStatus == StatusError || depStatus == StatusSkipped {
				result.Status = StatusSkipped
				if !found {
					result.Message = fmt.Sprintf("Skipped: dependency '%s' not found", depID)
				} else {
					result.Message = fmt.Sprintf("Skipped: dependency '%s' has status '%s'", depID, depStatus)
				}
				result.Duration = time.Since(startTime)
				return result
			}
		}
	}

	// Execute observations
	if e.config.Parallel && len(ctrl.Observations) > 1 {
		// Parallel execution of observations
		result.Observations = e.executeObservationsParallel(ctx, ctrl.Observations)
	} else {
		// Sequential execution of observations
		for _, obs := range ctrl.Observations {
			obsResult := e.executor.Execute(ctx, obs)
			result.Observations = append(result.Observations, obsResult)
		}
	}

	// Aggregate observation results to determine control status
	result.Status = aggregateControlStatus(result.Observations)

	// Generate message based on status
	result.Message = generateControlMessage(result.Status, result.Observations)

	// Set duration
	result.Duration = time.Since(startTime)

	return result
}

// shouldRun determines if a control should run based on the configuration filters.
func (e *Engine) shouldRun(ctrl config.Control) (bool, string) {
	// 0. EXCLUSIVE MODE: --control overrides all other filters
	if len(e.config.IncludeControlIDs) > 0 {
		if contains(e.config.IncludeControlIDs, ctrl.ID) {
			return true, ""
		}
		return false, "excluded by --control filter"
	}

	// 1. EXCLUSIONS: Check explicit exclusions first
	if contains(e.config.ExcludeControlIDs, ctrl.ID) {
		return false, "excluded by --exclude-control"
	}

	// 2. EXCLUSIONS: Check tag exclusions
	if len(e.config.ExcludeTags) > 0 {
		for _, tag := range ctrl.Tags {
			if contains(e.config.ExcludeTags, tag) {
				return false, fmt.Sprintf("excluded by --exclude-tags %s", tag)
			}
		}
	}

	// 3. INCLUDES: Check Severity filter (OR within list)
	if len(e.config.IncludeSeverities) > 0 {
		if !contains(e.config.IncludeSeverities, ctrl.Severity) {
			return false, "excluded by --severity filter"
		}
	}

	// 4. INCLUDES: Check Tags filter (OR within list - any match)
	if len(e.config.IncludeTags) > 0 {
		match := false
		for _, tag := range ctrl.Tags {
			if contains(e.config.IncludeTags, tag) {
				match = true
				break
			}
		}
		if !match {
			return false, "excluded by --tags filter"
		}
	}

	// 5. ADVANCED: Check Expression Filter
	if e.config.FilterProgram != nil {
		env := ControlEnv{
			ID:       ctrl.ID,
			Name:     ctrl.Name,
			Severity: ctrl.Severity,
			Owner:    ctrl.Owner,
			Tags:     ctrl.Tags,
		}
		output, err := expr.Run(e.config.FilterProgram, env)
		if err != nil {
			return false, fmt.Sprintf("filter expression error: %v", err)
		}
		if !output.(bool) {
			return false, "excluded by --filter expression"
		}
	}

	return true, ""
}

// contains checks if a string is present in a slice.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// executeObservationsParallel executes observations in parallel with concurrency limits.
func (e *Engine) executeObservationsParallel(ctx context.Context, observations []config.Observation) []ObservationResult {
	g, ctx := errgroup.WithContext(ctx)

	// Apply concurrency limit if specified
	if e.config.MaxConcurrentObservations > 0 {
		g.SetLimit(e.config.MaxConcurrentObservations)
	}

	// Pre-allocate results slice to exact size needed
	// Each goroutine writes to a unique index - no mutex needed
	results := make([]ObservationResult, len(observations))

	// Execute each observation in parallel
	for i, obs := range observations {
		g.Go(func() error {
			obsResult := e.executor.Execute(ctx, obs)
			// Safe without mutex: each goroutine writes to unique index in pre-allocated slice
			results[i] = obsResult
			return nil // Don't fail fast on individual observation errors
		})
	}

	// Wait for all observations to complete
	_ = g.Wait() // Ignore error as we don't fail fast

	return results
}

// aggregateControlStatus determines overall control status from observations.
// CRITICAL: Proven failures take precedence over errors for compliance reporting.
// Logic:
//   - If any observation is StatusFail → Control is StatusFail (proven non-compliance)
//   - If any observation is StatusError (but no failures) → Control is StatusError (inconclusive)
//   - If all observations are StatusPass → Control is StatusPass
//
// Rationale: If 9 observations FAIL and 1 errors, the control FAILED (not errored).
// A proven compliance violation is more important than a technical error.
// Auditors need to see definitive failures, not have them masked by errors.
func aggregateControlStatus(observations []ObservationResult) Status {
	if len(observations) == 0 {
		return StatusError
	}

	hasError := false
	hasFail := false
	passCount := 0

	for _, obs := range observations {
		switch obs.Status {
		case StatusError:
			hasError = true
		case StatusFail:
			hasFail = true
		case StatusPass:
			passCount++
		}
	}

	// CRITICAL: Failures take precedence over errors
	// If we proved non-compliance, that's what matters
	if hasFail {
		return StatusFail
	}

	// Errors only matter if we don't have proven failures
	if hasError {
		return StatusError
	}

	// All observations passed
	if passCount == len(observations) {
		return StatusPass
	}

	// Fallback (shouldn't reach here)
	return StatusError
}

// generateControlMessage generates a human-readable message for the control result.
func generateControlMessage(status Status, observations []ObservationResult) string {
	switch status {
	case StatusPass:
		if len(observations) == 1 {
			return "Check passed"
		}
		return fmt.Sprintf("All %d checks passed", len(observations))

	case StatusFail:
		failCount := 0
		for _, obs := range observations {
			if obs.Status == StatusFail {
				failCount++
			}
		}
		if failCount == 1 {
			return "1 check failed"
		}
		return fmt.Sprintf("%d checks failed", failCount)

	case StatusError:
		errorCount := 0
		for _, obs := range observations {
			if obs.Status == StatusError {
				errorCount++
			}
		}
		if errorCount == 1 {
			// Return the specific error message
			for _, obs := range observations {
				if obs.Status == StatusError && obs.Error != nil {
					return obs.Error.Message
				}
			}
			return "Check encountered an error"
		}
		return fmt.Sprintf("%d checks encountered errors", errorCount)

	case StatusSkipped:
		return "Skipped due to failed dependency"

	default:
		return "Unknown status"
	}
}

// Runtime returns the WASM runtime for accessing plugin schemas.
// This is used for pre-flight schema validation.
func (e *Engine) Runtime() *wasm.Runtime {
	return e.runtime
}

// Close closes the engine and releases resources.
func (e *Engine) Close(ctx context.Context) error {
	return e.runtime.Close(ctx)
}
