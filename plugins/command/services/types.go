package services

// ExecuteInput defines the input for command execution.
type ExecuteInput struct {
	// Shell command (mutually exclusive with Command)
	Run string `json:"run,omitempty" jsonschema:"description=Shell command to execute"`
	// Direct command (mutually exclusive with Run)
	Command string   `json:"command,omitempty" jsonschema:"description=Command to execute directly"`
	Args    []string `json:"args,omitempty" jsonschema:"description=Command arguments"`
	// Options
	Dir            string            `json:"dir,omitempty" jsonschema:"description=Working directory"`
	Env            map[string]string `json:"env,omitempty" jsonschema:"description=Environment variables"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty" jsonschema:"default=60,description=Execution timeout"`
}

// ExecuteOutput contains command execution results.
type ExecuteOutput struct {
	Command    string `json:"command" jsonschema:"description=Executed command"`
	ExitCode   int    `json:"exit_code" jsonschema:"description=Process exit code"`
	Stdout     string `json:"stdout" jsonschema:"description=Standard output"`
	Stderr     string `json:"stderr" jsonschema:"description=Standard error"`
	DurationMs int64  `json:"duration_ms" jsonschema:"description=Execution time in milliseconds"`
}
