package main

import (
	"github.com/spf13/cobra"
)

// pluginsCmd represents the plugins command
var pluginsCmd = &cobra.Command{
	Use:   "plugins",
	Short: "Manage plugins",
	Long:  `Manage plugins for Reglet using OCI registries. Pull, list, push, and prune plugins.`,
}

func init() {
	rootCmd.AddCommand(pluginsCmd)
}
