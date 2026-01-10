//go:build !wasip1

// Package exec provides command execution capabilities for WASM plugins.
// This stub file provides type definitions for non-WASM builds.
package exec

import (
	"context"
	"errors"
	"time"
)

// ErrNotWASM is returned when exec functions are called outside WASM environment.
var ErrNotWASM = errors.New("exec: not available outside WASM environment")

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
	Stdout    string
	Stderr    string
	ExitCode  int
	Duration  time.Duration // Execution duration
	IsTimeout bool          // True if command timed out
}

// Run is a stub that returns an error when called outside WASM.
func Run(ctx context.Context, req CommandRequest) (*CommandResponse, error) {
	_ = ctx
	_ = req
	return nil, ErrNotWASM
}
