package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet-sdk/go/infrastructure/wasm"
	"github.com/reglet-dev/reglet/plugins/command/core"

	// Import services to trigger auto-registration
	_ "github.com/reglet-dev/reglet/plugins/command/services"
)

type commandPlugin struct{}

func (p *commandPlugin) Manifest(ctx context.Context) (*entities.Manifest, error) {
	return core.Plugin.Manifest(), nil
}

func (p *commandPlugin) Check(ctx context.Context, configBytes []byte) (*entities.Result, error) {
	// Parse config
	var cfgStruct core.CommandConfig
	if err := json.Unmarshal(configBytes, &cfgStruct); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	cfg := &cfgStruct

	// Determine operation
	opName := "run" // Default op
	if cfg.ExpectedOutput != "" {
		opName = "output"
	} else if cfg.ExpectedExit != 0 || cfg.Run != "" || cfg.Command != "" {
		// If explicit check is requested, maybe map to specific op?
		// But "run" also checks exit code 0 by default if we want strictness?
		// services/command.go checks exit code for 'run' too if implementing legacy logic which defaulted to exit code 0 check for success.
		// Let's rely on services.runCommand logic.
		opName = "run"
	}

	// However, services.command.go defines: Run matches "RunHandler", ValidateExit -> "ValidateExitHandler", ValidateOutput -> "ValidateOutputHandler".
	// Core plugin defines generic "command" service with ops.
	// Let's map strict intentions if possible, or fallback to Run.
	// If expected_output is set, clearly output validation.
	// If expected_exit != 0 (non-default), maybe ValidateExit?
	// But legacy checked generic success (exit 0) unless configured otherwise.

	if cfg.ExpectedOutput != "" {
		opName = "output"
	} else if cfg.ExpectedExit != 0 {
		opName = "exit_code"
	} else {
		opName = "run"
	}

	// Map to actual op names in service
	// Service Op desc:
	// Run -> Run
	// ValidateExit -> ValidateExit
	// ValidateOutput -> ValidateOutput

	// Wait, runCommand takes 'mode' string "run", "exit_code", "output".
	// The handlers are: RunHandler, ValidateExitHandler, ValidateOutputHandler.

	var handlerName string
	switch opName {
	case "output":
		handlerName = "validate_output"
	case "exit_code":
		handlerName = "validate_exit"
	default:
		handlerName = "run"
	}

	handler, ok := core.Plugin.GetHandler("execution", handlerName)
	if !ok {
		return entities.ResultErrorPtr("configuration", fmt.Sprintf("Unknown operation: %s (service: execution)", handlerName)), nil
	}

	req := &plugin.Request{
		Client: wasm.NewExecAdapter(),
		Config: cfg,
		Raw:    configBytes,
	}

	return handler(ctx, req)
}

func init() {
	plugin.Register(&commandPlugin{})
}

func main() {}
