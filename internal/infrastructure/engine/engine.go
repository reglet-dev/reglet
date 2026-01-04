// Package engine is the engine for the app
package engine

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"github.com/expr-lang/expr/vm"
	"github.com/whiskeyjimbo/reglet/internal/domain/capabilities"
	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
	"github.com/whiskeyjimbo/reglet/internal/domain/execution"
	"github.com/whiskeyjimbo/reglet/internal/domain/repositories"
	"github.com/whiskeyjimbo/reglet/internal/domain/services"
	"github.com/whiskeyjimbo/reglet/internal/domain/values"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/build"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/redaction"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/wasm"
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

// DefaultExecutionConfig returns sensible defaults for parallel execution.
func DefaultExecutionConfig() ExecutionConfig {
	numCPU := runtime.NumCPU()

	// Default to NumCPU for controls, but at least 4 even on small systems
	maxControls := numCPU
	if maxControls < 4 {
		maxControls = 4
	}

	// Observations are within a control, so we use a smaller multiple of NumCPU
	maxObs := numCPU / 2
	if maxObs < 2 {
		maxObs = 2
	}
	if maxObs > 10 {
		maxObs = 10 // Cap observations per control to avoid too much nesting
	}

	return ExecutionConfig{
		MaxConcurrentControls:     maxControls,
		MaxConcurrentObservations: maxObs,
		Parallel:                  true,
	}
}

// Engine coordinates profile execution.
type Engine struct {
	runtime    *wasm.Runtime
	executor   *ObservationExecutor
	config     ExecutionConfig
	repository repositories.ExecutionResultRepository // Optional repository for persistence
	version    build.Info
}

// CapabilityManager defines the interface for capability management
type CapabilityManager interface {
	CollectRequiredCapabilities(ctx context.Context, profile *entities.Profile, runtime *wasm.Runtime, pluginDir string) (map[string][]capabilities.Capability, error)
	GrantCapabilities(required map[string][]capabilities.Capability) (map[string][]capabilities.Capability, error)
}

// NewEngine creates a new execution engine with default configuration.
func NewEngine(ctx context.Context, version build.Info) (*Engine, error) {
	return NewEngineWithConfig(ctx, version, DefaultExecutionConfig())
}

// NewEngineWithCapabilities creates an engine with interactive capability prompts
// and optional repository support.
func NewEngineWithCapabilities(
	ctx context.Context,
	version build.Info,
	capMgr CapabilityManager,
	pluginDir string,
	profile *entities.Profile,
	cfg ExecutionConfig,
	redactor *redaction.Redactor,
	repo repositories.ExecutionResultRepository, // Optional repository
	memoryLimitMB int,
) (*Engine, error) {
	// Create temporary runtime with no capabilities to load plugins and get requirements
	tempRuntime, err := wasm.NewRuntime(ctx, version)
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

	// Create WASM runtime with granted capabilities and redactor
	// SECURITY: Redactor prevents secrets from leaking to plugin stdout/stderr
	runtime, err := wasm.NewRuntimeWithCapabilities(ctx, version, granted, redactor, memoryLimitMB)
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
		runtime:    runtime,
		executor:   executor,
		config:     cfg,
		repository: repo,
		version:    version,
	}, nil
}

// NewEngineWithConfig creates a new execution engine with custom configuration.
func NewEngineWithConfig(ctx context.Context, version build.Info, cfg ExecutionConfig) (*Engine, error) {
	// Create WASM runtime with no capabilities (legacy path)
	runtime, err := wasm.NewRuntime(ctx, version)
	if err != nil {
		return nil, fmt.Errorf("failed to create WASM runtime: %w", err)
	}

	// Create observation executor with no redactor
	executor := NewObservationExecutor(runtime, nil)

	return &Engine{
		runtime:  runtime,
		executor: executor,
		config:   cfg,
		version:  version,
	}, nil
}

// Execute runs a complete profile and returns the result.
func (e *Engine) Execute(ctx context.Context, profile *entities.Profile) (*execution.ExecutionResult, error) {
	// Create execution result
	result := execution.NewExecutionResult(profile.Metadata.Name, profile.Metadata.Version)
	result.RegletVersion = e.version.String()

	// Calculate required dependencies if enabled
	var requiredControls map[string]bool
	if e.config.IncludeDependencies {
		var err error
		requiredControls, err = e.resolveDependencies(profile)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve dependencies: %w", err)
		}
	}

	// Execute controls
	if e.config.Parallel && len(profile.Controls.Items) > 1 {
		// Parallel execution of controls
		if err := e.executeControlsWithWorkerPool(ctx, profile.Controls.Items, result, requiredControls); err != nil {
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

	// Persist result if repository is configured
	if e.repository != nil {
		if err := e.repository.Save(ctx, result); err != nil {
			// Log error but don't fail execution
			slog.Warn("failed to persist execution result", "error", err, "execution_id", result.GetID())
		}
	}

	return result, nil
}

// resolveDependencies calculates the transitive closure of dependencies for matched controls.
func (e *Engine) resolveDependencies(profile *entities.Profile) (map[string]bool, error) {
	resolver := services.NewDependencyResolver()
	allDependencies, err := resolver.ResolveDependencies(profile.Controls.Items)
	if err != nil {
		return nil, err
	}

	required := make(map[string]bool)

	// Identify initial targets (controls that match filters)
	for _, ctrl := range profile.Controls.Items {
		if should, _ := e.shouldRun(ctrl); should {
			// Add all transitive dependencies for this control to required set
			if deps, ok := allDependencies[ctrl.ID]; ok {
				for depID := range deps {
					required[depID] = true
				}
			}
		}
	}

	return required, nil
}

// workerPoolState manages the state of dependency-aware parallel execution.
// Instead of organizing controls into levels with barriers, this approach
// maintains a dynamic ready queue and executes controls as soon as their
// dependencies are satisfied.
type workerPoolState struct {
	// Immutable after initialization (safe for concurrent reads)
	controlByID map[string]entities.Control // Control lookup by ID
	reverseDeps map[string][]string         // Control ID → list of dependent control IDs

	// Mutable state (owned by coordinator goroutine)
	inDegree      map[string]int  // Control ID → count of unmet dependencies
	readyQueue    []string        // Control IDs ready to execute (dependencies satisfied)
	completed     map[string]bool // Control IDs that have completed execution
	totalControls int             // Total number of controls to execute

	// Channels for work distribution and completion signaling
	workChan chan string // Buffered channel for control IDs ready to execute
	doneChan chan string // Unbuffered channel for completion signals

	// Context and error handling
	ctx      context.Context
	cancel   context.CancelFunc
	errGroup *errgroup.Group

	// Execution dependencies
	engine       *Engine
	execResult   *execution.ExecutionResult
	requiredDeps map[string]bool
}

// initializeWorkerPoolState builds the dependency graph and prepares initial state.
// It validates dependencies, detects cycles, and creates the initial ready queue.
func (e *Engine) initializeWorkerPoolState(
	ctx context.Context,
	controls []entities.Control,
	result *execution.ExecutionResult,
	requiredDeps map[string]bool,
) (*workerPoolState, error) {
	// Build dependency graph structures
	controlByID := make(map[string]entities.Control)
	inDegree := make(map[string]int)
	reverseDeps := make(map[string][]string)

	for _, ctrl := range controls {
		controlByID[ctrl.ID] = ctrl
		inDegree[ctrl.ID] = len(ctrl.DependsOn)

		// Build reverse dependency map (control → dependents)
		for _, depID := range ctrl.DependsOn {
			reverseDeps[depID] = append(reverseDeps[depID], ctrl.ID)
		}
	}

	// Validate all dependencies exist
	for _, ctrl := range controls {
		for _, depID := range ctrl.DependsOn {
			if _, exists := controlByID[depID]; !exists {
				return nil, fmt.Errorf("control %s depends on non-existent control %s", ctrl.ID, depID)
			}
		}
	}

	// Detect cycles using existing DependencyResolver
	resolver := services.NewDependencyResolver()
	if _, err := resolver.BuildControlDAG(controls); err != nil {
		return nil, err // Returns cycle error if detected
	}

	// Build initial ready queue (controls with no dependencies)
	readyQueue := []string{}
	for _, ctrl := range controls {
		if inDegree[ctrl.ID] == 0 {
			readyQueue = append(readyQueue, ctrl.ID)
		}
	}

	// Create channels
	// workChan is buffered to reduce blocking when workers finish
	// doneChan is unbuffered to ensure completion handling completes before worker continues
	maxConcurrent := e.config.MaxConcurrentControls
	if maxConcurrent <= 0 {
		maxConcurrent = 1 // Ensure at least 1 for buffer size
	}
	workChan := make(chan string, maxConcurrent)
	doneChan := make(chan string)

	// Create context and errgroup
	groupCtx, cancel := context.WithCancel(ctx)
	g, gCtx := errgroup.WithContext(groupCtx)
	// Do not set g.SetLimit here. The number of goroutines is controlled by 
	// numWorkers + 1 (coordinator) in executeControlsWithWorkerPool.
	// Setting it to MaxConcurrentControls would cause deadlock if numWorkers == MaxConcurrentControls.

	return &workerPoolState{
		controlByID:   controlByID,
		reverseDeps:   reverseDeps,
		inDegree:      inDegree,
		readyQueue:    readyQueue,
		completed:     make(map[string]bool),
		totalControls: len(controls),
		workChan:      workChan,
		doneChan:      doneChan,
		ctx:           gCtx,
		cancel:        cancel,
		errGroup:      g,
		engine:        e,
		execResult:    result,
		requiredDeps:  requiredDeps,
	}, nil
}

// enqueueReadyControls sends ready controls from the queue to the work channel.
// This is called by the coordinator after dependency updates.
// It sends as many controls as the channel can accept without blocking.
func (state *workerPoolState) enqueueReadyControls() {
	for len(state.readyQueue) > 0 {
		select {
		case state.workChan <- state.readyQueue[0]:
			// Successfully sent to worker, remove from queue
			state.readyQueue = state.readyQueue[1:]
		default:
			// Channel full (all workers busy), stop trying
			return
		}
	}
}

// handleControlCompletion processes a completed control by updating dependents.
// When a control completes, it decrements the in-degree of all dependent controls.
// Any dependent that reaches in-degree 0 becomes ready to execute.
func (state *workerPoolState) handleControlCompletion(controlID string) {
	// Mark as completed
	state.completed[controlID] = true

	// Update each dependent control
	for _, dependentID := range state.reverseDeps[controlID] {
		// Decrement in-degree (one less unmet dependency)
		state.inDegree[dependentID]--

		// If all dependencies are now satisfied, add to ready queue
		if state.inDegree[dependentID] == 0 {
			state.readyQueue = append(state.readyQueue, dependentID)
		}
	}
}

// coordinateExecution is the central coordinator that manages control execution.
// It runs in a single goroutine, owning all mutable state (inDegree, readyQueue).
// This design eliminates the need for fine-grained locking.
//
// Flow:
// 1. Enqueue initial batch of ready controls (in-degree 0)
// 2. Wait for completion signals on doneChan
// 3. Update dependencies and enqueue newly-ready controls
// 4. Repeat until all controls complete
func (state *workerPoolState) coordinateExecution() error {
	// Enqueue initial batch of ready controls
	state.enqueueReadyControls()

	completedCount := 0

	for completedCount < state.totalControls {
		select {
		case controlID := <-state.doneChan:
			// Control completed, update state
			completedCount++

			// Update dependents and find newly-ready controls
			state.handleControlCompletion(controlID)

			// Enqueue newly-ready controls to work channel
			state.enqueueReadyControls()

		case <-state.ctx.Done():
			// Context cancelled (user interrupt or timeout)
			return state.ctx.Err()
		}
	}

	// All controls completed, signal workers to exit
	close(state.workChan)
	return nil
}

// executeWorker runs in a goroutine, pulling controls from workChan and executing them.
// Workers are stateless - all state is in workerPoolState.
// Workers exit when workChan is closed by the coordinator.
func (state *workerPoolState) executeWorker() {
	for controlID := range state.workChan {
		// Lookup control (immutable map, safe for concurrent reads)
		ctrl, exists := state.controlByID[controlID]
		if !exists {
			// Should never happen (validated during initialization)
			continue
		}

		// Execute control using existing executeControl method
		// This handles dependency checking, observation execution, and status aggregation
		controlResult := state.engine.executeControl(
			state.ctx,
			ctrl,
			state.execResult,
			state.requiredDeps,
		)

		// Store result (thread-safe via ExecutionResult.AddControlResult mutex)
		state.execResult.AddControlResult(controlResult)

		// Signal completion to coordinator
		select {
		case state.doneChan <- controlID:
			// Completion signaled successfully
		case <-state.ctx.Done():
			// Context cancelled, exit worker
			return
		}
	}
}

// executeControlsWithWorkerPool executes controls in parallel using a dependency-aware worker pool.
// This replaces the level-based barrier approach with a more efficient strategy that executes
// controls as soon as their dependencies are satisfied.
//
// Algorithm:
// 1. Initialize state: build dependency graph, validate, create ready queue
// 2. Launch worker goroutines: pull from workChan, execute, signal completion
// 3. Launch coordinator goroutine: handle completions, update dependencies, enqueue ready controls
// 4. Wait for all goroutines to complete
//
// Performance: Controls execute immediately when dependencies satisfied (no level barriers).
func (e *Engine) executeControlsWithWorkerPool(
	ctx context.Context,
	controls []entities.Control,
	result *execution.ExecutionResult,
	requiredDeps map[string]bool,
) error {
	// Initialize worker pool state (build dependency graph, validate)
	state, err := e.initializeWorkerPoolState(ctx, controls, result, requiredDeps)
	if err != nil {
		return fmt.Errorf("failed to initialize worker pool: %w", err)
	}
	defer state.cancel() // Ensure cleanup on exit

	// Determine number of workers
	numWorkers := e.config.MaxConcurrentControls
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
		if numWorkers < 4 {
			numWorkers = 4
		}
	}

	// Launch worker goroutines
	for i := 0; i < numWorkers; i++ {
		state.errGroup.Go(func() error {
			state.executeWorker()
			return nil // Workers don't fail-fast, they record errors in results
		})
	}

	// Launch coordinator goroutine
	state.errGroup.Go(func() error {
		return state.coordinateExecution()
	})

	// Wait for all workers and coordinator to complete
	if err := state.errGroup.Wait(); err != nil {
		return fmt.Errorf("worker pool execution failed: %w", err)
	}

	return nil
}

// executeControl executes a single control and returns its result.
func (e *Engine) executeControl(ctx context.Context, ctrl entities.Control, execResult *execution.ExecutionResult, requiredDeps map[string]bool) execution.ControlResult {
	startTime := time.Now()

	result := execution.ControlResult{
		ID:           ctrl.ID,
		Name:         ctrl.Name,
		Description:  ctrl.Description,
		Severity:     ctrl.Severity,
		Tags:         ctrl.Tags,
		Observations: make([]execution.ObservationResult, 0, len(ctrl.Observations)),
	}

	// Check if control should run (filtering)
	shouldRun, skipReason := e.shouldRun(ctrl)

	// If filtering says skip, check if it's required as a dependency
	if !shouldRun && e.config.IncludeDependencies && requiredDeps[ctrl.ID] {
		shouldRun = true
		skipReason = "" // Clear skip reason as we are running it
	}

	if !shouldRun {
		result.Status = values.StatusSkipped
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
			if !found || depStatus == values.StatusFail || depStatus == values.StatusError || depStatus == values.StatusSkipped {
				result.Status = values.StatusSkipped
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
	// Extract statuses from observations
	observationStatuses := make([]values.Status, len(result.Observations))
	for i, obs := range result.Observations {
		observationStatuses[i] = obs.Status
	}

	aggregator := services.NewStatusAggregator()
	result.Status = aggregator.AggregateControlStatus(observationStatuses)

	// Generate message based on status
	result.Message = generateControlMessage(result.Status, result.Observations)

	// Set duration
	result.Duration = time.Since(startTime)

	return result
}

// shouldRun determines if a control should run based on the configuration filters.
func (e *Engine) shouldRun(ctrl entities.Control) (bool, string) {
	filter := services.NewControlFilter().
		WithExclusiveControls(e.config.IncludeControlIDs).
		WithExcludedControls(e.config.ExcludeControlIDs).
		WithExcludedTags(e.config.ExcludeTags).
		WithIncludedTags(e.config.IncludeTags).
		WithIncludedSeverities(e.config.IncludeSeverities).
		WithFilterExpression(e.config.FilterProgram)

	return filter.ShouldRun(ctrl)
}

// executeObservationsParallel executes observations in parallel with concurrency limits.
func (e *Engine) executeObservationsParallel(ctx context.Context, observations []entities.ObservationDefinition) []execution.ObservationResult {
	g, ctx := errgroup.WithContext(ctx)

	// Apply concurrency limit if specified
	if e.config.MaxConcurrentObservations > 0 {
		g.SetLimit(e.config.MaxConcurrentObservations)
	}

	// Pre-allocate results slice to exact size needed
	// Each goroutine writes to a unique index - no mutex needed
	results := make([]execution.ObservationResult, len(observations))

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

// generateControlMessage generates a human-readable message for the control result.
func generateControlMessage(status values.Status, observations []execution.ObservationResult) string {
	switch status {
	case values.StatusPass:
		if len(observations) == 1 {
			return "Check passed"
		}
		return fmt.Sprintf("All %d checks passed", len(observations))

	case values.StatusFail:
		failCount := 0
		for _, obs := range observations {
			if obs.Status == values.StatusFail {
				failCount++
			}
		}
		if failCount == 1 {
			return "1 check failed"
		}
		return fmt.Sprintf("%d checks failed", failCount)

	case values.StatusError:
		errorCount := 0
		for _, obs := range observations {
			if obs.Status == values.StatusError {
				errorCount++
			}
		}
		if errorCount == 1 {
			// Return the specific error message
			for _, obs := range observations {
				if obs.Status == values.StatusError && obs.Error != nil {
					return obs.Error.Message
				}
			}
			return "Check encountered an error"
		}
		return fmt.Sprintf("%d checks encountered errors", errorCount)

	case values.StatusSkipped:
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
