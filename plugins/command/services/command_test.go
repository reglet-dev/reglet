package services

import (
	"context"
	"errors"
	"testing"

	"github.com/reglet-dev/reglet-sdk/application/plugin"
	"github.com/reglet-dev/reglet-sdk/domain/ports"
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

func TestExamples(t *testing.T) {
	t.Skip("Requires mock injection setup for examples")
}

func TestCommandService_Execute_Run_Success(t *testing.T) {
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
	input := &ExecuteInput{Run: "echo hello"}

	ctx := context.Background()
	ctx = plugin.WithClient(ctx, ports.CommandRunner(mockRunner))

	output, err := svc.ExecuteHandler(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, "hello", output.Stdout)
	assert.Equal(t, 0, output.ExitCode)
}

func TestCommandService_Execute_Direct_Success(t *testing.T) {
	mockRunner := &mockCommandRunner{
		RunFunc: func(ctx context.Context, req ports.CommandRequest) (*ports.CommandResult, error) {
			assert.Equal(t, "/bin/ls", req.Command)
			assert.Equal(t, []string{"-la"}, req.Args)
			return &ports.CommandResult{
				ExitCode: 0,
			}, nil
		},
	}

	svc := &CommandService{}
	input := &ExecuteInput{
		Command: "/bin/ls",
		Args:    []string{"-la"},
	}

	ctx := context.Background()
	ctx = plugin.WithClient(ctx, ports.CommandRunner(mockRunner))

	output, err := svc.ExecuteHandler(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, 0, output.ExitCode)
}

func TestCommandService_Execute_ExecFailure(t *testing.T) {
	mockRunner := &mockCommandRunner{
		RunFunc: func(ctx context.Context, req ports.CommandRequest) (*ports.CommandResult, error) {
			return nil, errors.New("exec failed")
		},
	}

	svc := &CommandService{}
	input := &ExecuteInput{Run: "foobar"}

	ctx := context.Background()
	ctx = plugin.WithClient(ctx, ports.CommandRunner(mockRunner))

	output, err := svc.ExecuteHandler(ctx, input)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "execution failed")
}

func TestCommandService_MutualExclusivity(t *testing.T) {
	svc := &CommandService{}
	// Both run and command
	input := &ExecuteInput{
		Run:     "foo",
		Command: "bar",
	}

	ctx := context.Background()
	ctx = plugin.WithClient(ctx, &mockCommandRunner{}) // Mock needed to retrieve client? Yes, GetClient panics if missing.

	output, err := svc.ExecuteHandler(ctx, input)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "cannot specify both")
}
