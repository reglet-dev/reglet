package main

import (
	"context"
	"fmt"
	"strings"

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
		return regletsdk.ConfigError(err), nil
	}

	if cfg.Run == "" && cfg.Command == "" {
		return regletsdk.ConfigError(fmt.Errorf("either 'run' or 'command' must be specified")), nil
	}

	var cmd string
	var args []string

	// "run" mode: execute via shell
	if cfg.Run != "" {
		// Default to /bin/sh for now (Linux/Unix target)
		// Ideally, we detect OS or allow config override
		cmd = "/bin/sh"
		args = []string{"-c", cfg.Run}
	} else {
		// "command" mode: direct execution
		cmd = cfg.Command
		args = cfg.Args
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

	result := map[string]interface{}{
		"stdout":        stdoutTrimmed,
		"stderr":        stderrTrimmed,
		"exit_code":     resp.ExitCode,
		"stdout_raw":    resp.Stdout, // Keep raw for regex matching if needed
		"stderr_raw":    resp.Stderr,
		"status":        resp.ExitCode == 0, // Status determines pass/fail
	}

	return regletsdk.Success(result), nil
}
