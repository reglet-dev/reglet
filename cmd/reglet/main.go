package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
)

const (
	version = "0.1.0-dev"
)

func main() {
	// Setup structured logging
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Create context
	ctx := context.Background()

	// Run CLI
	if err := run(ctx, os.Args[1:]); err != nil {
		slog.Error("command failed", "error", err)
		os.Exit(1)
	}
}

// run executes the CLI with the given arguments
func run(ctx context.Context, args []string) error {
	// Phase 1b: Simple command parsing
	// For now, we only support "reglet check <profile>"

	if len(args) == 0 {
		return fmt.Errorf("no command specified\n\nUsage: reglet check <profile.yaml> [flags]")
	}

	command := args[0]

	switch command {
	case "check":
		return runCheck(ctx, args[1:])
	case "version", "--version", "-v":
		fmt.Printf("reglet version %s\n", version)
		return nil
	case "help", "--help", "-h":
		printHelp()
		return nil
	default:
		return fmt.Errorf("unknown command: %s\n\nRun 'reglet help' for usage", command)
	}
}

// printHelp prints usage information
func printHelp() {
	fmt.Println("reglet - Compliance and infrastructure validation")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  reglet check <profile.yaml> [flags]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  check       Execute compliance checks from a profile")
	fmt.Println("  version     Print version information")
	fmt.Println("  help        Show this help message")
	fmt.Println()
	fmt.Println("Flags for 'check' command:")
	fmt.Println("  --format string    Output format: table, json, yaml (default: table)")
	fmt.Println("  --output string    Output file path (default: stdout)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  reglet check profile.yaml")
	fmt.Println("  reglet check profile.yaml --format json")
	fmt.Println("  reglet check profile.yaml --format json --output results.json")
	fmt.Println()
}
