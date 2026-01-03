package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/charmbracelet/huh"
	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
	"github.com/whiskeyjimbo/reglet/internal/domain/entities"
)

type AWSInitOptions struct {
	Region        string
	Services      []string
	Framework     string
	OutputPath    string
	RunCheck      bool
	IncludeTags   []string
	ExcludeTags   []string
	NoInteractive bool
}

func runAWSInit(cmd *cobra.Command) error {
	opts := AWSInitOptions{}
	var err error

	opts.Region, _ = cmd.Flags().GetString("region")
	opts.Services, _ = cmd.Flags().GetStringSlice("services")
	opts.Framework, _ = cmd.Flags().GetString("framework")
	opts.OutputPath, _ = cmd.Flags().GetString("output")
	opts.RunCheck, _ = cmd.Flags().GetBool("run-check")
	opts.IncludeTags, _ = cmd.Flags().GetStringSlice("include-tags")
	opts.ExcludeTags, _ = cmd.Flags().GetStringSlice("exclude-tags")
	opts.NoInteractive, _ = cmd.Flags().GetBool("no-interactive")

	ctx := context.Background()

	// Load default AWS config to get region if not provided
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	if opts.Region == "" && awsCfg.Region != "" {
		opts.Region = awsCfg.Region
	}

	if !opts.NoInteractive {
		if opts.Region == "" {
			err = huh.NewInput().
				Title("AWS Region").
				Value(&opts.Region).
				Run()
			if err != nil {
				return err
			}
		}

		if len(opts.Services) == 0 {
			err = huh.NewMultiSelect[string]().
				Title("Select services to scan").
				Options(
					huh.NewOption("EC2 (Instances, Security Groups)", "ec2").Selected(true),
					huh.NewOption("S3 (Buckets, Encryption)", "s3").Selected(true),
					huh.NewOption("IAM (Users, Roles, Policies)", "iam"),
					huh.NewOption("VPC (Network Configuration)", "vpc").Selected(true),
				).
				Value(&opts.Services).
				Run()
			if err != nil {
				return err
			}
		}

		if opts.Framework == "" {
			err = huh.NewSelect[string]().
				Title("Select compliance framework").
				Options(
					huh.NewOption("Custom (scan all resources)", "custom"),
					huh.NewOption("CIS AWS Foundations Benchmark", "cis"),
					huh.NewOption("SOC2", "soc2"),
					huh.NewOption("ISO27001", "iso27001"),
				).
				Value(&opts.Framework).
				Run()
			if err != nil {
				return err
			}
		}
	}

	// Update config with selected region if different
	if opts.Region != awsCfg.Region {
		awsCfg.Region = opts.Region
	}

	// 3. Scan AWS resources
	fmt.Printf("Scanning AWS resources in %s...\n", opts.Region)
	scanner := NewAWSScanner(awsCfg)
	resources, err := scanner.Scan(ctx, opts.Services)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// 4. Generate profile
	generator := NewProfileGenerator(opts.Framework, opts.Region)
	profile, err := generator.Generate(resources)
	if err != nil {
		return fmt.Errorf("profile generation failed: %w", err)
	}

	// 5. Save profile
	fmt.Printf("Generated profile with %d controls\n", len(profile.Controls.Items))
	if err := saveProfile(profile, opts.OutputPath); err != nil {
		return fmt.Errorf("failed to save profile: %w", err)
	}
	
	fmt.Printf("✓ Profile saved to %s\n", opts.OutputPath)

	if opts.RunCheck {
		fmt.Printf("Run 'reglet check %s' to execute checks.\n", opts.OutputPath)
	}

	return nil
}

func saveProfile(profile *entities.Profile, path string) error {
	data, err := yaml.Marshal(profile)
	if err != nil {
		return err
	}
		return os.WriteFile(path, data, 0644)
	}
	