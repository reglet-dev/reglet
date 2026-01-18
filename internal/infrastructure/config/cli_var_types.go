// Package config provides configuration parsing and variable handling.
package config

// CLIVarSource indicates the source of a CLI variable value.
type CLIVarSource int

const (
	// CLIVarSourceFlag indicates a value from --set key=value.
	CLIVarSourceFlag CLIVarSource = iota
	// CLIVarSourceFile indicates a value from --set-file key=filepath.
	CLIVarSourceFile
	// CLIVarSourceEnv indicates a value from --set-env key=ENV_VAR.
	CLIVarSourceEnv
)

// String returns a human-readable name for the source.
func (s CLIVarSource) String() string {
	switch s {
	case CLIVarSourceFlag:
		return "flag"
	case CLIVarSourceFile:
		return "file"
	case CLIVarSourceEnv:
		return "env"
	default:
		return "unknown"
	}
}

// CLIVariable represents a parsed CLI variable with its source and type information.
type CLIVariable struct {
	// Key is the variable path using dot notation (e.g., "paths.config").
	Key string
	// Value is the typed value (string, int64, float64, or bool).
	Value interface{}
	// RawValue is the original string value from the CLI.
	RawValue string
	// Source indicates where the value came from.
	Source CLIVarSource
}
