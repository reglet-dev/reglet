package core

import "time"

// CommandConfig defines the configuration for command execution checks.
type CommandConfig struct {
	Run     string   `json:"run,omitempty" jsonschema:"description=Command string to execute via shell"`
	Command string   `json:"command,omitempty" jsonschema:"description=Executable path"`
	Args    []string `json:"args,omitempty" jsonschema:"description=Arguments"`
	Dir     string   `json:"dir,omitempty" jsonschema:"description=Working directory"`
	Env     []string `json:"env,omitempty" jsonschema:"description=Environment variables"`

	// Validation fields
	ExpectedExit   int    `json:"expected_exit,omitempty" jsonschema:"default=0,description=Expected exit code"`
	ExpectedOutput string `json:"expected_output,omitempty" jsonschema:"description=Expected string in stdout"`

	// Timeout configuration
	TimeoutSeconds int `json:"timeout_seconds,omitempty" jsonschema:"default=30,description=Execution timeout in seconds"`

	// Internal
	Timeout time.Duration `json:"-"`
}
