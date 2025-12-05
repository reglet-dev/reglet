package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of reglet",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("reglet version %s\n", version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
