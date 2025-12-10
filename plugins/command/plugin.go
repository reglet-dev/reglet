package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	regletsdk "github.com/whiskeyjimbo/reglet/sdk"
	"github.com/whiskeyjimbo/reglet/sdk/exec"
)

// commandPlugin implements the sdk.Plugin interface.
type commandPlugin struct{}

// Describe returns plugin metadata.
func (p *commandPlugin) Describe(ctx context.Context) (regletsdk.Metadata, error) {
	return regletsdk.Metadata{
		Name:        "command",
		Version:     "1.0.0",
		Description: "Execute commands and validate output",
		Capabilities: []regletsdk.Capability{
			{
				Kind:    "exec",
				Pattern: "**", // Plugin requests general exec permission; user grants specific
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
	Timeout int      `json:"timeout,omitempty" default:"30" description:"Execution timeout in seconds"`
}

// Schema returns the JSON schema for the plugin's configuration.
func (p *commandPlugin) Schema(ctx context.Context) ([]byte, error) {
	return regletsdk.GenerateSchema(CommandConfig{})
}

// Check executes the command observation.
func (p *commandPlugin) Check(ctx context.Context, config regletsdk.Config) (regletsdk.Evidence, error) {
	var cfg CommandConfig
	if err := regletsdk.ValidateConfig(config, &cfg); err != nil {
		return regletsdk.Evidence{
			Status: false,
			Error:  regletsdk.ToErrorDetail(&regletsdk.ConfigError{Err: err}),
		}, nil
	}

	// Validate mutual exclusivity
	if cfg.Run == "" && cfg.Command == "" {
		return regletsdk.Evidence{
			Status: false,
			Error: regletsdk.ToErrorDetail(&regletsdk.ConfigError{
				Err: fmt.Errorf("either 'run' or 'command' must be specified"),
			}),
		}, nil
	}
	if cfg.Run != "" && cfg.Command != "" {
		return regletsdk.Evidence{
			Status: false,
			Error: regletsdk.ToErrorDetail(&regletsdk.ConfigError{
				Err: fmt.Errorf("cannot specify both 'run' and 'command' - choose one"),
			}),
		}, nil
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
		//
		// Safe:   run: "systemctl is-active sshd"
		// Unsafe: run: "echo " + userInput  (if userInput can contain shell metacharacters)
		cmd = "/bin/sh"
		args = []string{"-c", cfg.Run}
		execMode = "shell"
	} else {
		// "command" mode: direct execution (safer - no shell interpretation)
		cmd = cfg.Command
		args = cfg.Args
		execMode = "direct"
	}

	resp, err := exec.Run(ctx, exec.CommandRequest{
		Command: cmd,
		Args:    args,
		Dir:     cfg.Dir,
		Env:     cfg.Env,
		Timeout: cfg.Timeout,
	})

	if err != nil {
		return regletsdk.Failure("exec", fmt.Sprintf("execution failed: %v", err)), nil
	}

	// Clean output (trim whitespace)
	stdoutTrimmed := strings.TrimSpace(resp.Stdout)
	stderrTrimmed := strings.TrimSpace(resp.Stderr)

	// Determine status based on exit code
	statusPass := resp.ExitCode == 0

	result := map[string]interface{}{
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
		"command":        cmd,       // Actual command executed
		"args":           args,      // Actual arguments used
		"working_dir":    cfg.Dir,
		"timeout_config": cfg.Timeout,
	}

	// Add original command for clarity
	if execMode == "shell" {
		result["shell_command"] = cfg.Run
	} else {
		result["command_path"] = cfg.Command
		result["command_args"] = cfg.Args
	}

	// Return Evidence with Status based on exit code
	return regletsdk.Evidence{
		Status:    statusPass,
		Data:      result,
		Timestamp: time.Now(),
	}, nil
}
