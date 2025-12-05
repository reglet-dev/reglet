package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of reglet",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("reglet version %s\n", version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
