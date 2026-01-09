//go:build !wasip1

// Package exec provides command execution capabilities for WASM plugins.
// This stub file provides type definitions for non-WASM builds.
package exec

import (
	"context"
	"errors"
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
	Stdout     string
	Stderr     string
	ExitCode   int
	DurationMs int64 // Execution duration in milliseconds
	IsTimeout  bool  // True if command timed out
}

// Run is a stub that returns an error when called outside WASM.
func Run(_ context.Context, _ CommandRequest) (*CommandResponse, error) {
	return nil, ErrNotWASM
}
