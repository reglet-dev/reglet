package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/reglet-dev/reglet/internal/infrastructure/container"
	"github.com/spf13/cobra"
)

// CommandContext provides common command dependencies.
// Eliminates repetitive container initialization across CLI commands.
type CommandContext struct {
	Container *container.Container
	Logger    *slog.Logger
	Context   context.Context
}

// CommandHandler is a function that executes with initialized dependencies.
// Commands focus on business logic, not infrastructure setup.
type CommandHandler func(*CommandContext, *cobra.Command, []string) error

// withContainer wraps a command handler with container initialization.
// Handles common setup: config loading, logger creation, dependency injection.
//
// Usage:
//
//	cmd := &cobra.Command{
//	    Use: "list",
//	    RunE: withContainer(func(ctx *CommandContext, cmd *cobra.Command, args []string) error {
//	        // Direct service access, no boilerplate
//	        return ctx.Container.PluginService().ListCachedPlugins(ctx.Context)
//	    }),
//	}
func withContainer(handler CommandHandler) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		// Get config path from flag
		configPath, _ := cmd.Flags().GetString("config")

		// Initialize logger
		logger := slog.Default()

		// Initialize container with dependencies
		c, err := container.New(container.Options{
			SystemConfigPath: configPath,
			Logger:           logger,
		})
		if err != nil {
			return fmt.Errorf("failed to initialize application: %w", err)
		}

		// Create command context
		ctx := &CommandContext{
			Container: c,
			Logger:    logger,
			Context:   cmd.Context(),
		}

		// Execute handler
		return handler(ctx, cmd, args)
	}
}

// addCommonFlags adds standard flags to a command.
// Ensures consistent flag naming across all commands.
func addCommonFlags(cmd *cobra.Command) {
	cmd.Flags().String("config", "", "Path to config file")
}
