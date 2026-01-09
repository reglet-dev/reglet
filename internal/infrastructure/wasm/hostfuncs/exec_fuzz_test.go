package hostfuncs

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/reglet-dev/reglet/wireformat"
)

// FuzzExecRequestParsing fuzzes exec request wire format parsing
// TARGETS: JSON unmarshaling of ExecRequestWire, command parsing, args handling
// EXPECTED FAILURES: Malformed JSON, invalid UTF-8, extreme field sizes
func FuzzExecRequestParsing(f *testing.F) {
	// Seed with valid exec request structures
	validReq := wireformat.ExecRequestWire{
		Command: "/usr/bin/ls",
		Args:    []string{"-la", "/tmp"},
		Dir:     "/home/user",
		Env:     []string{"PATH=/usr/bin", "HOME=/home/user"},
	}

	validJSON, _ := json.Marshal(validReq)
	f.Add(validJSON)

	// Shell execution patterns (security-sensitive)
	f.Add([]byte(`{"command":"bash","args":["-c","echo hello"]}`))
	f.Add([]byte(`{"command":"/bin/sh","args":["-c","whoami"]}`))
	f.Add([]byte(`{"command":"zsh","args":["-c","id"]}`))

	// Interpreter code execution patterns
	f.Add([]byte(`{"command":"python","args":["-c","import os"]}`))
	f.Add([]byte(`{"command":"python3","args":["--command","print('hi')"]}`))
	f.Add([]byte(`{"command":"perl","args":["-e","print 1"]}`))
	f.Add([]byte(`{"command":"ruby","args":["-e","puts 1"]}`))
	f.Add([]byte(`{"command":"node","args":["--eval","console.log(1)"]}`))

	// AWK with BEGIN/END blocks
	f.Add([]byte(`{"command":"awk","args":["BEGIN{print 1}","/etc/passwd"]}`))
	f.Add([]byte(`{"command":"gawk","args":["END {print NR}","/var/log/syslog"]}`))

	// Edge cases for path traversal in command
	f.Add([]byte(`{"command":"../../../bin/sh","args":[]}`))
	f.Add([]byte(`{"command":"/bin/../bin/bash","args":["-c","id"]}`))

	// Large inputs
	f.Add([]byte(`{"command":"` + strings.Repeat("a", 100000) + `"}`))
	f.Add([]byte(`{"args":["` + strings.Repeat("x", 100000) + `"]}`))

	// Empty and null fields
	f.Add([]byte(`{"command":"","args":[]}`))
	f.Add([]byte(`{"command":"ls","args":null}`))
	f.Add([]byte(`{}`))

	// Special characters in command/args
	f.Add([]byte(`{"command":"ls","args":["\u0000","\n","\r"]}`))
	f.Add([]byte(`{"command":"echo","args":["$(whoami)"]}`))
	f.Add([]byte(`{"command":"cat","args":["file;rm -rf /"]}`))

	// Environment variable injection
	f.Add([]byte(`{"command":"ls","env":["LD_PRELOAD=/tmp/evil.so"]}`))
	f.Add([]byte(`{"command":"ls","env":["PATH=","HOME="]}`))

	f.Fuzz(func(t *testing.T, jsonData []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PANIC on input: %v", r)
			}
		}()

		var req wireformat.ExecRequestWire
		if err := json.Unmarshal(jsonData, &req); err != nil {
			return // Invalid JSON is expected, not a bug
		}

		// Exercise the security-sensitive detection functions (unexported but accessible in test)
		_ = detectExecutionType(req.Command, req.Args)
		_ = isShellExecution(req.Command)
		_ = isKnownInterpreter(req.Command)
		_ = hasCodeExecutionFlags(req.Command, req.Args)
		_ = hasSuspiciousFlags(req.Args)
		_ = getBasename(req.Command)
	})
}

// FuzzExecutionTypeDetection specifically targets the execution type detection logic
// TARGETS: detectExecutionType, isShellExecution, hasCodeExecutionFlags, hasSuspiciousFlags
// EXPECTED FAILURES: None - these should handle any input gracefully
func FuzzExecutionTypeDetection(f *testing.F) {
	// Valid commands
	f.Add("ls", "-la")
	f.Add("/usr/bin/cat", "/etc/passwd")

	// Shell commands
	f.Add("bash", "-c")
	f.Add("sh", "-c")
	f.Add("/bin/zsh", "-c")

	// Interpreters with code execution flags
	f.Add("python", "-c")
	f.Add("python3.11", "--command")
	f.Add("perl", "-e")
	f.Add("ruby", "-e")
	f.Add("node", "--eval")
	f.Add("lua5.4", "-e")

	// AWK patterns
	f.Add("awk", "BEGIN{")
	f.Add("gawk", "END {")

	// Edge cases
	f.Add("", "")
	f.Add("/", "")
	f.Add("///", "///")
	f.Add(strings.Repeat("a", 10000), strings.Repeat("b", 10000))

	f.Fuzz(func(t *testing.T, command string, firstArg string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PANIC on command=%q, firstArg=%q: %v", command, firstArg, r)
			}
		}()

		args := []string{firstArg}

		// Exercise all detection functions - none should panic
		_ = detectExecutionType(command, args)
		_ = isShellExecution(command)
		_ = isKnownInterpreter(command)
		_ = hasCodeExecutionFlags(command, args)
		_ = hasSuspiciousFlags(args)
		_ = getBasename(command)

		// Also test with empty and nil-like args
		_ = detectExecutionType(command, nil)
		_ = detectExecutionType(command, []string{})
		_ = hasCodeExecutionFlags(command, nil)
		_ = hasSuspiciousFlags(nil)
	})
}
