//go:build !wasip1

package exec

import (
	"encoding/json"
	"testing"

	"github.com/reglet-dev/reglet/wireformat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: Actual command execution requires WASM runtime with host functions.
// These tests focus on wire format structures and data serialization.

func TestCommandRequest_Serialization(t *testing.T) {
	tests := []struct {
		name    string
		request wireformat.ExecRequestWire
	}{
		{
			name: "simple command",
			request: wireformat.ExecRequestWire{
				Command: "/usr/bin/whoami",
			},
		},
		{
			name: "command with args",
			request: wireformat.ExecRequestWire{
				Command: "/bin/echo",
				Args:    []string{"hello", "world"},
			},
		},
		{
			name: "command with environment",
			request: wireformat.ExecRequestWire{
				Command: "/usr/bin/env",
				Env:     []string{"FOO=bar", "BAZ=qux"},
			},
		},
		{
			name: "command with working directory",
			request: wireformat.ExecRequestWire{
				Command: "/usr/bin/ls",
				Args:    []string{"-la"},
				Dir:     "/tmp",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.request)
			require.NoError(t, err)

			var decoded wireformat.ExecRequestWire
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.request.Command, decoded.Command)
			assert.Equal(t, tt.request.Args, decoded.Args)
			assert.Equal(t, tt.request.Env, decoded.Env)
			assert.Equal(t, tt.request.Dir, decoded.Dir)
		})
	}
}

func TestCommandResponse_Serialization(t *testing.T) {
	tests := []struct {
		name     string
		response wireformat.ExecResponseWire
	}{
		{
			name: "successful execution",
			response: wireformat.ExecResponseWire{
				Stdout:     "command output",
				Stderr:     "",
				ExitCode:   0,
				DurationMs: 123,
			},
		},
		{
			name: "failed execution",
			response: wireformat.ExecResponseWire{
				Stdout:   "",
				Stderr:   "error: command not found",
				ExitCode: 127,
			},
		},
		{
			name: "timeout execution",
			response: wireformat.ExecResponseWire{
				Stdout:     "partial output",
				Stderr:     "",
				ExitCode:   -1,
				DurationMs: 5000,
				IsTimeout:  true,
			},
		},
		{
			name: "execution with mixed output",
			response: wireformat.ExecResponseWire{
				Stdout:     "normal output\n",
				Stderr:     "warning message\n",
				ExitCode:   0,
				DurationMs: 456,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.response)
			require.NoError(t, err)

			var decoded wireformat.ExecResponseWire
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.response.Stdout, decoded.Stdout)
			assert.Equal(t, tt.response.Stderr, decoded.Stderr)
			assert.Equal(t, tt.response.ExitCode, decoded.ExitCode)
			assert.Equal(t, tt.response.DurationMs, decoded.DurationMs)
			assert.Equal(t, tt.response.IsTimeout, decoded.IsTimeout)
		})
	}
}

func TestCommandExitCodes(t *testing.T) {
	exitCodes := []struct {
		code    int
		meaning string
	}{
		{0, "success"},
		{1, "general error"},
		{2, "misuse of shell command"},
		{126, "command cannot execute"},
		{127, "command not found"},
		{130, "terminated by Ctrl+C"},
		{137, "killed (SIGKILL)"},
		{143, "terminated (SIGTERM)"},
	}

	for _, ec := range exitCodes {
		t.Run(ec.meaning, func(t *testing.T) {
			resp := wireformat.ExecResponseWire{
				ExitCode: ec.code,
			}

			data, err := json.Marshal(resp)
			require.NoError(t, err)

			var decoded wireformat.ExecResponseWire
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)
			assert.Equal(t, ec.code, decoded.ExitCode)
		})
	}
}

func TestCommandWithWireformatError(t *testing.T) {
	// Test that CommandResponse can include structured errors
	// (though typically errors are in the ExitCode/Stderr)

	// Note: This documents how errors could be structured
	// Current implementation uses ExitCode + Stderr for errors
	tests := []struct {
		name      string
		errorType string
		errorCode string
		message   string
	}{
		{
			name:      "permission denied",
			errorType: "execution",
			errorCode: "EACCES",
			message:   "permission denied",
		},
		{
			name:      "timeout",
			errorType: "timeout",
			errorCode: "ETIMEDOUT",
			message:   "command execution timed out",
		},
		{
			name:      "not found",
			errorType: "execution",
			errorCode: "ENOENT",
			message:   "command not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Documenting that errors would use ErrorDetail structure
			errorDetail := &wireformat.ErrorDetail{
				Type:    tt.errorType,
				Code:    tt.errorCode,
				Message: tt.message,
			}

			data, err := json.Marshal(errorDetail)
			require.NoError(t, err)

			var decoded wireformat.ErrorDetail
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.errorType, decoded.Type)
			assert.Equal(t, tt.errorCode, decoded.Code)
			assert.Equal(t, tt.message, decoded.Message)
		})
	}
}

func TestCommandDurationTracking(t *testing.T) {
	// Test that duration is properly tracked in ExecResponseWire
	durations := []int64{0, 10, 100, 1000, 5000, 30000}

	for _, duration := range durations {
		t.Run(string(rune(duration)), func(t *testing.T) {
			resp := wireformat.ExecResponseWire{
				ExitCode:   0,
				DurationMs: duration,
			}

			data, err := json.Marshal(resp)
			require.NoError(t, err)

			var decoded wireformat.ExecResponseWire
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)
			assert.Equal(t, duration, decoded.DurationMs)
		})
	}
}
