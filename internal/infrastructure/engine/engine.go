// Package engine coordinates profile execution and validation.
package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/reglet-dev/reglet/internal/domain/capabilities"
	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/execution"
	"github.com/reglet-dev/reglet/internal/domain/repositories"
	"github.com/reglet-dev/reglet/internal/domain/services"
	"github.com/reglet-dev/reglet/internal/infrastructure/build"
	"github.com/reglet-dev/reglet/internal/infrastructure/sensitivedata"
	"github.com/reglet-dev/reglet/internal/infrastructure/wasm"
)

// ObservationExecutable defines the interface for executing observations.
type ObservationExecutable interface {
	Execute(ctx context.Context, obs entities.ObservationDefinition) execution.ObservationResult
}

// Engine coordinates profile execution.
type Engine struct {
	repository repositories.ExecutionResultRepository
	executor   ObservationExecutable
	truncator  execution.TruncationStrategy
	runtime    *wasm.Runtime
	version    build.Info
	config     ExecutionConfig
}

// CapabilityCollector collects required capabilities from plugins.
type CapabilityCollector interface {
	CollectRequiredCapabilities(ctx context.Context, profile entities.ProfileReader, runtime *wasm.Runtime, pluginDir string) (map[string][]capabilities.Capability, error)
}

// CapabilityGranter grants capabilities (interactively or automatically).
type CapabilityGranter interface {
	GrantCapabilities(required map[string][]capabilities.Capability) (map[string][]capabilities.Capability, error)
}

// CapabilityManager combines collection and granting for convenience.
type CapabilityManager interface {
	CapabilityCollector
	CapabilityGranter
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
	profile entities.ProfileReader,
	cfg ExecutionConfig,
	redactor *sensitivedata.Redactor,
	repo repositories.ExecutionResultRepository,
	memoryLimitMB int,
	truncator execution.TruncationStrategy,
) (*Engine, error) {
	// Create temporary runtime with no capabilities to load plugins and get requirements
	tempRuntime, err := wasm.NewRuntime(ctx, version)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary runtime: %w", err)
	}

	// Collect required capabilities from all plugins
	required, err := capMgr.CollectRequiredCapabilities(ctx, profile, tempRuntime, pluginDir)
	if err != nil {
		_ = tempRuntime.Close(ctx)
		return nil, fmt.Errorf("failed to collect capabilities: %w", err)
	}

	_ = tempRuntime.Close(ctx)

	// Get granted capabilities (will prompt user if needed)
	granted, err := capMgr.GrantCapabilities(required)
	if err != nil {
		return nil, fmt.Errorf("failed to grant capabilities: %w", err)
	}

	// Create WASM runtime with granted capabilities and redactor
	runtime, err := wasm.NewRuntimeWithCapabilities(ctx, version, granted, redactor, memoryLimitMB)
	if err != nil {
		return nil, fmt.Errorf("failed to create WASM runtime: %w", err)
	}

	executor := NewExecutor(runtime,
		WithPluginDir(pluginDir),
		WithRedactor(redactor),
	)

	// Preload plugins for schema validation
	for _, ctrl := range profile.GetAllControls() {
		for _, obs := range ctrl.ObservationDefinitions {
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
		truncator:  truncator,
	}, nil
}

// NewEngineWithConfig creates a new execution engine with custom configuration.
func NewEngineWithConfig(ctx context.Context, version build.Info, cfg ExecutionConfig) (*Engine, error) {
	runtime, err := wasm.NewRuntime(ctx, version)
	if err != nil {
		return nil, fmt.Errorf("failed to create WASM runtime: %w", err)
	}

	executor := NewExecutor(runtime) // Auto-detect plugin dir, no redactor

	return &Engine{
		runtime:   runtime,
		executor:  executor,
		config:    cfg,
		version:   version,
		truncator: &execution.GreedyTruncator{},
	}, nil
}

// Execute runs a complete profile and returns the result.
func (e *Engine) Execute(ctx context.Context, profile entities.ProfileReader) (*execution.ExecutionResult, error) {
	// Check context before starting
	if ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("execution timed out: %w", ctx.Err())
		}
		return nil, ctx.Err()
	}

	metadata := profile.GetMetadata()
	result := execution.NewExecutionResult(metadata.Name, metadata.Version)
	result.RegletVersion = e.version.String()

	var requiredControls map[string]bool
	if e.config.IncludeDependencies {
		var err error
		requiredControls, err = e.resolveDependencies(profile)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve dependencies: %w", err)
		}
	}

	allControls := profile.GetAllControls()
	if e.config.Parallel && len(allControls) > 1 {
		if err := e.executeControlsWithWorkerPool(ctx, allControls, result, requiredControls); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, fmt.Errorf("execution timed out: %w", err)
			}
			return nil, err
		}
	} else {
		for i, ctrl := range allControls {
			// Check context in loop
			if ctx.Err() != nil {
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
					return nil, fmt.Errorf("execution timed out: %w", ctx.Err())
				}
				return nil, ctx.Err()
			}

			controlResult := e.executeControl(ctx, ctrl, i, result, requiredControls)
			result.AddControlResult(controlResult)
		}

		if ctx.Err() != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, fmt.Errorf("execution timed out: %w", ctx.Err())
			}
			return nil, ctx.Err()
		}
	}

	result.Finalize()

	if e.repository != nil {
		if err := e.repository.Save(ctx, result); err != nil {
			slog.Warn("failed to persist execution result", "error", err, "execution_id", result.GetID())
		}
	}

	return result, nil
}

// resolveDependencies calculates the transitive closure of dependencies for matched controls.
func (e *Engine) resolveDependencies(profile entities.ProfileReader) (map[string]bool, error) {
	resolver := services.NewDependencyResolver()
	allControls := profile.GetAllControls()
	allDependencies, err := resolver.ResolveDependencies(allControls)
	if err != nil {
		return nil, err
	}

	required := make(map[string]bool)

	for _, ctrl := range allControls {
		if should, _ := e.shouldRun(ctrl); should {
			if deps, ok := allDependencies[ctrl.ID]; ok {
				for depID := range deps {
					required[depID] = true
				}
			}
		}
	}

	return required, nil
}

// Runtime returns the WASM runtime for accessing plugin schemas.
func (e *Engine) Runtime() *wasm.Runtime {
	return e.runtime
}

// Close closes the engine and releases resources.
func (e *Engine) Close(ctx context.Context) error {
	return e.runtime.Close(ctx)
}
