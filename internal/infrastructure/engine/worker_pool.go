package engine

import (
	"context"
	"fmt"
	"runtime"

	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
	"github.com/whiskeyjimbo/reglet/internal/domain/execution"
	"github.com/whiskeyjimbo/reglet/internal/domain/services"
	"golang.org/x/sync/errgroup"
)

// workerPoolState manages the state of dependency-aware parallel execution.
// Instead of organizing controls into levels with barriers, this approach
// maintains a dynamic ready queue and executes controls as soon as their
// dependencies are satisfied.
type workerPoolState struct {
	// Immutable after initialization (safe for concurrent reads)
	controlByID      map[string]entities.Control // Control lookup by ID
	controlIndexByID map[string]int              // Control ID → original definition order (for deterministic output)
	reverseDeps      map[string][]string         // Control ID → list of dependent control IDs

	// Mutable state (owned by coordinator goroutine)
	inDegree      map[string]int  // Control ID → count of unmet dependencies
	readyQueue    []string        // Control IDs ready to execute (dependencies satisfied)
	completed     map[string]bool // Control IDs that have completed execution
	totalControls int             // Total number of controls to execute

	// Channels for work distribution and completion signaling
	workChan chan string // Buffered channel for control IDs ready to execute
	doneChan chan string // Buffered channel for completion signals (prevents worker blocking)

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
	controlIndexByID := make(map[string]int) // Track original definition order
	inDegree := make(map[string]int)
	reverseDeps := make(map[string][]string)

	for i, ctrl := range controls {
		controlByID[ctrl.ID] = ctrl
		controlIndexByID[ctrl.ID] = i // Store original index for deterministic output
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
	// workChan is buffered to reduce blocking when coordinator sends work
	// doneChan is buffered to prevent workers from blocking when signaling completion
	maxConcurrent := e.config.MaxConcurrentControls
	if maxConcurrent <= 0 {
		maxConcurrent = 1 // Ensure at least 1 for buffer size
	}
	workChan := make(chan string, maxConcurrent)
	doneChan := make(chan string, len(controls))

	// Create context and errgroup
	groupCtx, cancel := context.WithCancel(ctx)
	g, gCtx := errgroup.WithContext(groupCtx)

	return &workerPoolState{
		controlByID:      controlByID,
		controlIndexByID: controlIndexByID,
		reverseDeps:      reverseDeps,
		inDegree:         inDegree,
		readyQueue:       readyQueue,
		completed:        make(map[string]bool),
		totalControls:    len(controls),
		workChan:         workChan,
		doneChan:         doneChan,
		ctx:              gCtx,
		cancel:           cancel,
		errGroup:         g,
		engine:           e,
		execResult:       result,
		requiredDeps:     requiredDeps,
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
	state.completed[controlID] = true

	for _, dependentID := range state.reverseDeps[controlID] {
		state.inDegree[dependentID]--

		if state.inDegree[dependentID] == 0 {
			state.readyQueue = append(state.readyQueue, dependentID)
		}
	}
}

// coordinateExecution is the central coordinator that manages control execution.
// It runs in a single goroutine, owning all mutable state (inDegree, readyQueue).
// This design eliminates the need for fine-grained locking.
func (state *workerPoolState) coordinateExecution() error {
	defer close(state.workChan)

	state.enqueueReadyControls()

	completedCount := 0

	for completedCount < state.totalControls {
		select {
		case controlID := <-state.doneChan:
			completedCount++
			state.handleControlCompletion(controlID)
			state.enqueueReadyControls()

		case <-state.ctx.Done():
			return state.ctx.Err()
		}
	}

	return nil
}

// executeWorker runs in a goroutine, pulling controls from workChan and executing them.
// Workers are stateless - all state is in workerPoolState.
// Workers exit when workChan is closed by the coordinator.
func (state *workerPoolState) executeWorker() {
	for controlID := range state.workChan {
		ctrl, exists := state.controlByID[controlID]
		if !exists {
			continue
		}

		index := state.controlIndexByID[controlID]

		controlResult := state.engine.executeControl(
			state.ctx,
			ctrl,
			index,
			state.execResult,
			state.requiredDeps,
		)

		state.execResult.AddControlResult(controlResult)

		select {
		case state.doneChan <- controlID:
		case <-state.ctx.Done():
			return
		}
	}
}

// executeControlsWithWorkerPool executes controls in parallel using a dependency-aware worker pool.
// Controls execute immediately when dependencies are satisfied (no level barriers).
func (e *Engine) executeControlsWithWorkerPool(
	ctx context.Context,
	controls []entities.Control,
	result *execution.ExecutionResult,
	requiredDeps map[string]bool,
) error {
	state, err := e.initializeWorkerPoolState(ctx, controls, result, requiredDeps)
	if err != nil {
		return fmt.Errorf("failed to initialize worker pool: %w", err)
	}
	defer state.cancel()

	numWorkers := e.config.MaxConcurrentControls
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
		if numWorkers < MinConcurrentControls {
			numWorkers = MinConcurrentControls
		}
	}

	for i := 0; i < numWorkers; i++ {
		state.errGroup.Go(func() error {
			state.executeWorker()
			return nil
		})
	}

	state.errGroup.Go(func() error {
		return state.coordinateExecution()
	})

	if err := state.errGroup.Wait(); err != nil {
		return fmt.Errorf("worker pool execution failed: %w", err)
	}

	return nil
}
