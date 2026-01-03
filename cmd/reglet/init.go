package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [provider]",
	Short: "Initialize a compliance profile from existing infrastructure",
	Long: `Generate a Reglet profile by scanning cloud infrastructure.

Supported providers:
  aws     Amazon Web Services`,
	Example: `  reglet init aws
  reglet init aws --region us-west-2`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

func init() {
	initCmd.Flags().String("region", "", "AWS region")
	initCmd.Flags().StringSlice("services", nil, "Services to scan (ec2, s3, iam, vpc)")
	initCmd.Flags().String("framework", "", "Compliance framework")
	initCmd.Flags().String("output", "reglet-aws-profile.yaml", "Output file path")
	initCmd.Flags().Bool("run-check", false, "Run check after generation")
	initCmd.Flags().StringSlice("include-tags", nil, "Include resources with tags")
	initCmd.Flags().StringSlice("exclude-tags", nil, "Exclude resources with tags")
	initCmd.Flags().Bool("no-interactive", false, "Disable interactive prompts")

	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}

	provider := args[0]
	switch provider {
	case "aws":
		return runAWSInit(cmd)
	default:
		return fmt.Errorf("unsupported provider: %s", provider)
	}
}
