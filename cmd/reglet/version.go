package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/whiskeyjimbo/reglet/internal/infrastructure/build"
)

// versionCmd implements the version command.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of reglet",
	Run: func(_ *cobra.Command, _ []string) {
		info := build.Get()
		fmt.Printf("reglet version %s\n", info.Full())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
