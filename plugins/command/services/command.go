package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
	"github.com/reglet-dev/reglet/plugins/command/core"
)

// CommandService provides command execution checks.
type CommandService struct {
	plugin.Service `name:"execution" desc:"Command execution and validation"`

	Run            plugin.Op `desc:"Execute command and return output" method:"RunHandler"`
	ValidateExit   plugin.Op `desc:"Execute and verify exit code" method:"ValidateExitHandler"`
	ValidateOutput plugin.Op `desc:"Execute and check output contains expected value" method:"ValidateOutputHandler"`
}

func init() {
	plugin.MustRegisterService(core.Plugin, &CommandService{})
}

// Handler implementations

func (s *CommandService) RunHandler(ctx context.Context, req *plugin.Request) (*entities.Result, error) {
	cfg := req.Config.(*core.CommandConfig)
	return runCommand(ctx, req, cfg, "run")
}

func (s *CommandService) ValidateExitHandler(ctx context.Context, req *plugin.Request) (*entities.Result, error) {
	cfg := req.Config.(*core.CommandConfig)
	return runCommand(ctx, req, cfg, "exit_code")
}

func (s *CommandService) ValidateOutputHandler(ctx context.Context, req *plugin.Request) (*entities.Result, error) {
	cfg := req.Config.(*core.CommandConfig)
	return runCommand(ctx, req, cfg, "output")
}

// runCommand executes the command and performs validation based on the operation mode.
func runCommand(ctx context.Context, req *plugin.Request, cfg *core.CommandConfig, mode string) (*entities.Result, error) {
	runner := req.Client.(ports.CommandRunner)

	// Logic from legacy Check method
	// Derive effective timeout
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = 30
	}
	timeoutLimit := cfg.TimeoutSeconds * 1000 // ms

	// Validate mutual exclusivity
	if cfg.Run == "" && cfg.Command == "" {
		return entities.ResultErrorPtr("config", "either 'run' or 'command' must be specified"), nil
	}
	if cfg.Run != "" && cfg.Command != "" {
		return entities.ResultErrorPtr("config", "cannot specify both 'run' and 'command' - choose one"), nil
	}

	var cmd string
	var args []string
	var execMode string

	if cfg.Run != "" {
		// Shell mode
		cmd = "/bin/sh"
		args = []string{"-c", cfg.Run}
		execMode = "shell"
	} else {
		// Direct execution
		cmd = cfg.Command
		args = cfg.Args
		execMode = "direct"
	}

	reqData := ports.CommandRequest{
		Command: cmd,
		Args:    args,
		Dir:     cfg.Dir,
		Env:     cfg.Env,
		Timeout: timeoutLimit,
	}

	start := time.Now()
	resp, err := runner.Run(ctx, reqData)
	if err != nil {
		return entities.ResultErrorPtr("execution_failed", fmt.Sprintf("execution failed: %v", err)), nil
	}
	_ = start // unused if using resp.DurationMs

	// Clean output
	stdoutTrimmed := strings.TrimSpace(resp.Stdout)
	stderrTrimmed := strings.TrimSpace(resp.Stderr)

	resultData := map[string]interface{}{
		"stdout":         stdoutTrimmed,
		"stderr":         stderrTrimmed,
		"stdout_raw":     resp.Stdout,
		"stderr_raw":     resp.Stderr,
		"exit_code":      resp.ExitCode,
		"duration_ms":    resp.DurationMs,
		"is_timeout":     resp.IsTimeout,
		"exec_mode":      execMode,
		"working_dir":    cfg.Dir,
		"timeout_config": timeoutLimit,
	}

	if execMode == "shell" {
		resultData["shell_command"] = cfg.Run
	} else {
		resultData["command_path"] = cfg.Command
		resultData["command_args"] = cfg.Args
	}

	// Validation Logic
	if mode == "exit_code" || mode == "run" {
		expectedExit := cfg.ExpectedExit
		if resp.ExitCode != expectedExit {
			return entities.ResultFailurePtr(
				fmt.Sprintf("Unexpected exit code: got %d, want %d", resp.ExitCode, expectedExit),
				resultData,
			), nil
		}
	}

	if mode == "output" {
		if cfg.ExpectedOutput != "" {
			if !strings.Contains(resp.Stdout, cfg.ExpectedOutput) && !strings.Contains(resp.Stderr, cfg.ExpectedOutput) {
				return entities.ResultFailurePtr(
					fmt.Sprintf("Output does not contain expected string: %q", cfg.ExpectedOutput),
					resultData,
				), nil
			}
		}
	}

	msg := "Command executed successfully"
	if mode == "run" {
		msg = fmt.Sprintf("Command exited with code %d", resp.ExitCode)
	}

	return entities.ResultSuccessPtr(msg, resultData), nil
}
