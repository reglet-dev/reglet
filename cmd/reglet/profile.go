// Package main provides the reglet CLI for compliance and infrastructure validation.
package main

import (
	"github.com/spf13/cobra"
)

// profileCmd is the parent command for profile management subcommands.
var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage remote profile cache",
	Long: `Commands for managing cached remote profiles.

Use these commands to list, update, and manage profiles fetched from remote sources.`,
}

func init() {
	rootCmd.AddCommand(profileCmd)
}
