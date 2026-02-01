package services

import (
	"context"
	"errors"
	"testing"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
	"github.com/reglet-dev/reglet/plugins/command/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCommandRunner struct {
	RunFunc func(ctx context.Context, req ports.CommandRequest) (*ports.CommandResult, error)
}

func (m *mockCommandRunner) Run(ctx context.Context, req ports.CommandRequest) (*ports.CommandResult, error) {
	if m.RunFunc != nil {
		return m.RunFunc(ctx, req)
	}
	return &ports.CommandResult{ExitCode: 0}, nil
}

func TestCommandService_Run_Success(t *testing.T) {
	mockRunner := &mockCommandRunner{
		RunFunc: func(ctx context.Context, req ports.CommandRequest) (*ports.CommandResult, error) {
			assert.Equal(t, "/bin/sh", req.Command)
			assert.Contains(t, req.Args, "echo hello")
			return &ports.CommandResult{
				Stdout:   "hello",
				ExitCode: 0,
			}, nil
		},
	}

	svc := &CommandService{}
	cfg := &core.CommandConfig{
		Run: "echo hello",
	}
	req := &plugin.Request{
		Client: mockRunner,
		Config: cfg,
	}

	result, err := svc.RunHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.ResultStatusSuccess, result.Status)
	assert.Equal(t, "hello", result.Data["stdout"])
}

func TestCommandService_Run_ExecFailure(t *testing.T) {
	mockRunner := &mockCommandRunner{
		RunFunc: func(ctx context.Context, req ports.CommandRequest) (*ports.CommandResult, error) {
			return nil, errors.New("exec failed")
		},
	}

	svc := &CommandService{}
	cfg := &core.CommandConfig{Run: "foobar"}
	req := &plugin.Request{Client: mockRunner, Config: cfg}

	result, err := svc.RunHandler(context.Background(), req)
	require.NoError(t, err)
	// Execution error maps to Error status in our service.go
	assert.Equal(t, entities.ResultStatusError, result.Status)
}

func TestCommandService_ValidateExit_Fail(t *testing.T) {
	mockRunner := &mockCommandRunner{
		RunFunc: func(ctx context.Context, req ports.CommandRequest) (*ports.CommandResult, error) {
			return &ports.CommandResult{
				ExitCode: 1,
			}, nil
		},
	}

	svc := &CommandService{}
	cfg := &core.CommandConfig{
		Run:          "fail",
		ExpectedExit: 0, // Expect 0, got 1
	}
	req := &plugin.Request{Client: mockRunner, Config: cfg}

	result, err := svc.ValidateExitHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.ResultStatusFailure, result.Status)
	assert.Contains(t, result.Message, "Unexpected exit code")
}

func TestCommandService_ValidateOutput_Success(t *testing.T) {
	mockRunner := &mockCommandRunner{
		RunFunc: func(ctx context.Context, req ports.CommandRequest) (*ports.CommandResult, error) {
			return &ports.CommandResult{
				Stdout:   "foobarbaz",
				ExitCode: 0,
			}, nil
		},
	}

	svc := &CommandService{}
	cfg := &core.CommandConfig{
		Run:            "echo foobarbaz",
		ExpectedOutput: "bar",
	}
	req := &plugin.Request{Client: mockRunner, Config: cfg}

	result, err := svc.ValidateOutputHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.ResultStatusSuccess, result.Status)
}

func TestCommandService_ValidateOutput_Fail(t *testing.T) {
	mockRunner := &mockCommandRunner{
		RunFunc: func(ctx context.Context, req ports.CommandRequest) (*ports.CommandResult, error) {
			return &ports.CommandResult{
				Stdout:   "nope",
				ExitCode: 0,
			}, nil
		},
	}

	svc := &CommandService{}
	cfg := &core.CommandConfig{
		Run:            "echo nope",
		ExpectedOutput: "yes",
	}
	req := &plugin.Request{Client: mockRunner, Config: cfg}

	result, err := svc.ValidateOutputHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.ResultStatusFailure, result.Status)
}

func TestCommandService_MutualExclusivity(t *testing.T) {
	svc := &CommandService{}
	// Both run and command
	cfg := &core.CommandConfig{
		Run:     "foo",
		Command: "bar",
	}
	req := &plugin.Request{
		Client: &mockCommandRunner{},
		Config: cfg,
	}

	result, err := svc.RunHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.ResultStatusError, result.Status)
	assert.Contains(t, result.Error.Message, "cannot specify both")
}

func TestCommandService_DirectCommand(t *testing.T) {
	mockRunner := &mockCommandRunner{
		RunFunc: func(ctx context.Context, req ports.CommandRequest) (*ports.CommandResult, error) {
			assert.Equal(t, "/bin/ls", req.Command)
			assert.Equal(t, []string{"-la"}, req.Args)
			return &ports.CommandResult{ExitCode: 0}, nil
		},
	}

	svc := &CommandService{}
	cfg := &core.CommandConfig{
		Command: "/bin/ls",
		Args:    []string{"-la"},
	}
	req := &plugin.Request{Client: mockRunner, Config: cfg}

	result, err := svc.RunHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.ResultStatusSuccess, result.Status)
}
