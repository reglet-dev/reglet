package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/reglet-dev/reglet-sdk/go/application/config"
	"github.com/reglet-dev/reglet-sdk/go/application/schema"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"

	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
	"github.com/reglet-dev/reglet-sdk/go/exec"
)

// commandPlugin implements the sdk.Plugin interface.
type commandPlugin struct {
	runner ports.CommandRunner
}

// Describe returns plugin metadata.
func (p *commandPlugin) Describe(ctx context.Context) (entities.Metadata, error) {
	return entities.Metadata{
		Name:        "command",
		Version:     "1.0.0",
		Description: "Execute commands and validate output",
		Capabilities: []entities.Capability{
			{
				Category: "exec",
				Resource: "**", // Plugin requests general exec permission; user grants specific
			},
		},
	}, nil
}

// CommandConfig represents the configuration for the command plugin.
type CommandConfig struct {
	Run     string   `json:"run,omitempty" description:"Command string to execute via shell"`
	Command string   `json:"command,omitempty" description:"Executable path"`
	Args    []string `json:"args,omitempty" description:"Arguments"`
	Dir     string   `json:"dir,omitempty" description:"Working directory"`
	Env     []string `json:"env,omitempty" description:"Environment variables"`
	// Timeout is the effective execution timeout.
	// It is derived from TimeoutSeconds (in seconds) after configuration validation.
	Timeout time.Duration `json:"-" description:"Execution timeout"`
	// TimeoutSeconds is the JSON-configurable timeout in seconds.
	TimeoutSeconds int `json:"timeout,omitempty" default:"30" description:"Execution timeout in seconds"`
}

// Schema returns the JSON schema for the plugin's configuration.
func (p *commandPlugin) Schema(ctx context.Context) ([]byte, error) {
	return schema.GenerateSchema(CommandConfig{})
}

// Check executes the command observation.
func (p *commandPlugin) Check(ctx context.Context, cfgRaw config.Config) (entities.Result, error) {
	var cfg CommandConfig
	if err := config.Validate(cfgRaw, &cfg); err != nil {
		return entities.ResultError(entities.NewErrorDetail("config", err.Error())), nil
	}

	// Derive the effective timeout as a time.Duration.
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = 30
	}
	cfg.Timeout = time.Duration(cfg.TimeoutSeconds) * time.Second

	// Validate mutual exclusivity
	if cfg.Run == "" && cfg.Command == "" {
		return entities.ResultError(entities.NewErrorDetail("config", "either 'run' or 'command' must be specified")), nil
	}
	if cfg.Run != "" && cfg.Command != "" {
		return entities.ResultError(entities.NewErrorDetail("config", "cannot specify both 'run' and 'command' - choose one")), nil
	}

	var cmd string
	var args []string
	var execMode string

	// "run" mode: execute via shell
	if cfg.Run != "" {
		// ⚠️  SECURITY WARNING: Shell execution can be dangerous!
		// - Requires explicit "exec:/bin/sh" capability (user must grant shell access)
		// - Vulnerable to command injection if Run contains untrusted input
		// - For untrusted input, use "command" mode with explicit args instead
		cmd = "/bin/sh"
		args = []string{"-c", cfg.Run}
		execMode = "shell"
	} else {
		// "command" mode: direct execution (safer - no shell interpretation)
		cmd = cfg.Command
		args = cfg.Args
		execMode = "direct"
	}

	start := time.Now()

	// Prepare options
	opts := []exec.RunOption{}
	if p.runner != nil {
		opts = append(opts, exec.WithRunner(p.runner))
	}

	resp, err := exec.Run(ctx, exec.CommandRequest{
		Command: cmd,
		Args:    args,
		Dir:     cfg.Dir,
		Env:     cfg.Env,
		Timeout: int(cfg.Timeout / time.Second), // Convert Duration to seconds
	}, opts...)
	metadata := entities.NewRunMetadata(start, time.Now())

	if err != nil {
		return entities.ResultFailure(fmt.Sprintf("execution failed: %v", err), nil).WithMetadata(metadata), nil
	}

	// Clean output (trim whitespace)
	stdoutTrimmed := strings.TrimSpace(resp.Stdout)
	stderrTrimmed := strings.TrimSpace(resp.Stderr)

	// Determine status based on exit code
	statusPass := resp.ExitCode == 0

	resultData := map[string]interface{}{
		// Output streams
		"stdout":     stdoutTrimmed,
		"stderr":     stderrTrimmed,
		"stdout_raw": resp.Stdout, // Keep raw for regex matching if needed
		"stderr_raw": resp.Stderr,

		// Execution results
		"exit_code":   resp.ExitCode,
		"duration_ms": resp.DurationMs,
		"is_timeout":  resp.IsTimeout,

		// Command metadata (for debugging and auditing)
		"exec_mode":      execMode, // "shell" or "direct"
		"command":        cmd,      // Actual command executed
		"args":           args,     // Actual arguments used
		"working_dir":    cfg.Dir,
		"timeout_config": cfg.Timeout,
	}

	// Add original command for clarity
	if execMode == "shell" {
		resultData["shell_command"] = cfg.Run
	} else {
		resultData["command_path"] = cfg.Command
		resultData["command_args"] = cfg.Args
	}

	if statusPass {
		return entities.ResultSuccess("Command executed successfully", resultData).WithMetadata(metadata), nil
	}

	return entities.ResultFailure(fmt.Sprintf("Command exited with code %d", resp.ExitCode), resultData).WithMetadata(metadata), nil
}
