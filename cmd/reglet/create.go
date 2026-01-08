package main

import (
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(newCreateCmd())
}

func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create new resources",
		Long:  `Create new plugins, profiles, or other Reglet resources.`,
	}

	cmd.AddCommand(newCreatePluginCmd())

	return cmd
}
