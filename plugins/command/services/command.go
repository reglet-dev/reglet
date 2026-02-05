package services

import (
	"context"
	"fmt"
	"time"

	"github.com/reglet-dev/reglet-sdk/application/plugin"
	"github.com/reglet-dev/reglet-sdk/domain/ports"
	"github.com/reglet-dev/reglet/plugins/command/core"
)

// CommandService provides command execution checks.
type CommandService struct {
	plugin.Service `name:"execution" desc:"Command execution and validation"`

	Execute plugin.Op[ExecuteInput, ExecuteOutput] `desc:"Execute command and return output" method:"ExecuteHandler"`
}

func init() {
	plugin.RegisterOp[ExecuteInput, ExecuteOutput]("Execute",
		plugin.Example[ExecuteInput, ExecuteOutput]{
			Name:        "echo_hello",
			Description: "Run echo hello",
			Input:       ExecuteInput{Run: "echo hello"},
			ExpectedOutput: &ExecuteOutput{
				Stdout:   "hello\n",
				ExitCode: 0,
			},
		},
	)

	plugin.MustRegisterService(core.Plugin, &CommandService{})
}

// ExecuteHandler executes the command.
func (s *CommandService) ExecuteHandler(ctx context.Context, in *ExecuteInput) (*ExecuteOutput, error) {
	runner := plugin.GetClient[ports.CommandRunner](ctx)

	// Validate mutual exclusivity
	if in.Run == "" && in.Command == "" {
		return nil, fmt.Errorf("config error: either 'run' or 'command' must be specified")
	}
	if in.Run != "" && in.Command != "" {
		return nil, fmt.Errorf("config error: cannot specify both 'run' and 'command'")
	}

	timeout := time.Duration(in.TimeoutSeconds) * time.Second
	if in.TimeoutSeconds <= 0 {
		timeout = 30 * time.Second
	}

	timeoutMs := int(timeout.Milliseconds())

	var cmd string
	var args []string

	if in.Run != "" {
		// Shell mode
		cmd = "/bin/sh"
		args = []string{"-c", in.Run}
	} else {
		// Direct execution
		cmd = in.Command
		args = in.Args
	}

	// Env is already []string, passed directly
	env := in.Env

	reqData := ports.CommandRequest{
		Command: cmd,
		Args:    args,
		Dir:     in.Dir,
		Env:     env,
		Timeout: timeoutMs,
	}

	start := time.Now()
	resp, err := runner.Run(ctx, reqData)
	if err != nil {
		return nil, fmt.Errorf("execution failed: %w", err)
	}

	return &ExecuteOutput{
		Command:    cmd,
		ExitCode:   resp.ExitCode,
		Stdout:     resp.Stdout,
		Stderr:     resp.Stderr,
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}
