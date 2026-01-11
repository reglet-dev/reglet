package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	pluginsCmd.AddCommand(newPluginsPruneCmd())
}

func newPluginsPruneCmd() *cobra.Command {
	var keepVersions int

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove old plugin versions",
		Long:  `Remove older versions of plugins from the local cache, keeping only the most recent ones.`,
		Example: `  # Keep last 3 versions of each plugin (default is 5)
  reglet plugins prune --keep 3`,
		Args: cobra.NoArgs,
		RunE: withContainer(func(ctx *CommandContext, cmd *cobra.Command, args []string) error {
			// Service call
			err := ctx.Container.PluginService().PruneCache(ctx.Context, keepVersions)
			if err != nil {
				return fmt.Errorf("failed to prune cache: %w", err)
			}

			fmt.Printf("Cache pruned. Kept %d latest versions per plugin.\n", keepVersions)
			return nil
		}),
	}

	cmd.Flags().IntVar(&keepVersions, "keep", 5, "Number of versions to keep per plugin")
	addCommonFlags(cmd)

	return cmd
}
