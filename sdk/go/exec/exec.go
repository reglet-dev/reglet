//go:build wasip1

package exec

import (
	"context"
	"encoding/json"
	"fmt"

	sdkcontext "github.com/whiskeyjimbo/reglet/sdk/internal/context"
	"github.com/whiskeyjimbo/reglet/sdk/internal/abi"
	"github.com/whiskeyjimbo/reglet/wireformat"
)

//go:wasmimport reglet_host exec_command
func host_exec_command(reqPacked uint64) uint64

// CommandRequest defines the parameters for executing a command.
type CommandRequest struct {
	Command string
	Args    []string
	Dir     string
	Env     []string
	Timeout int // seconds
}

// CommandResponse contains the result of the command execution.
type CommandResponse struct {
	Stdout     string
	Stderr     string
	ExitCode   int
	DurationMs int64 // Execution duration in milliseconds
	IsTimeout  bool  // True if command timed out
}

// Run executes a command on the host system.
// Requires "exec:<command>" capability.
func Run(ctx context.Context, req CommandRequest) (*CommandResponse, error) {
	// 1. Prepare wire request with context
	wireReq := wireformat.ExecRequestWire{
		Context: sdkcontext.ContextToWire(ctx),
		Command: req.Command,
		Args:    req.Args,
		Dir:     req.Dir,
		Env:     req.Env,
	}

	reqData, err := json.Marshal(wireReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 2. Send to host
	reqPacked := abi.PtrFromBytes(reqData)
	defer abi.DeallocatePacked(reqPacked)

	resPacked := host_exec_command(reqPacked)

	// 3. Read response
	resBytes := abi.BytesFromPtr(resPacked)
	if resBytes == nil {
		// This might happen if host returns 0 packed (null/empty), which indicates error usually handled inside hostWriteResponse but if 0 returned...
		// Wait, hostWriteResponse returns 0 on alloc failure.
		return nil, fmt.Errorf("host returned null response")
	}
	defer abi.DeallocatePacked(resPacked) // Free host-allocated response memory

	var wireRes wireformat.ExecResponseWire
	if err := json.Unmarshal(resBytes, &wireRes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// 4. Handle errors
	if wireRes.Error != nil {
		return nil, wireRes.Error
	}

	return &CommandResponse{
		Stdout:     wireRes.Stdout,
		Stderr:     wireRes.Stderr,
		ExitCode:   wireRes.ExitCode,
		DurationMs: wireRes.DurationMs,
		IsTimeout:  wireRes.IsTimeout,
	}, nil
}
