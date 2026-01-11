package main

import (
	"fmt"

	"github.com/reglet-dev/reglet/internal/application/dto"
	"github.com/spf13/cobra"
)

func init() {
	pluginsCmd.AddCommand(newPluginsPullCmd())
}

func newPluginsPullCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pull <reference>",
		Short: "Pull a plugin from a registry",
		Long:  `Pull a plugin from an OCI registry and store it in the local cache.`,
		Example: `  # Pull a plugin by version
  reglet plugins pull ghcr.io/reglet-dev/plugins/aws:1.0.0

  # Pull validation
  reglet plugins pull ghcr.io/reglet-dev/plugins/aws:1.0.0 --config reglet.yaml`,
		Args: cobra.ExactArgs(1),
		RunE: withContainer(func(ctx *CommandContext, cmd *cobra.Command, args []string) error {
			refStr := args[0]

			// Create spec
			spec := &dto.PluginSpecDTO{
				Name: refStr,
			}

			// Service call
			path, err := ctx.Container.PluginService().LoadPlugin(ctx.Context, spec)
			if err != nil {
				return fmt.Errorf("failed to pull plugin: %w", err)
			}

			fmt.Printf("Plugin pulled successfully: %s\n", path)
			return nil
		}),
	}

	addCommonFlags(cmd)

	return cmd
}
