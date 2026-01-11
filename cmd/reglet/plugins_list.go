package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func init() {
	pluginsCmd.AddCommand(newPluginsListCmd())
}

func newPluginsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List cached plugins",
		Long:    `List all plugins currently available in the local cache.`,
		Example: `  reglet plugins list`,
		Args:    cobra.NoArgs,
		RunE: withContainer(func(ctx *CommandContext, cmd *cobra.Command, args []string) error {
			// Service call
			plugins, err := ctx.Container.PluginService().ListCachedPlugins(ctx.Context)
			if err != nil {
				return fmt.Errorf("failed to list plugins: %w", err)
			}

			// Render output
			if len(plugins) == 0 {
				fmt.Println("No plugins found in cache.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			if _, err := fmt.Fprintln(w, "REGISTRY\tNAME\tVERSION\tDIGEST"); err != nil {
				return fmt.Errorf("failed to write header: %w", err)
			}

			for _, p := range plugins {
				ref := p.Reference()
				digest := p.Digest().String()
				// Truncate digest
				if len(digest) > 12 {
					digest = digest[:12]
				}

				if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					ref.Registry(),
					ref.Name(),
					ref.Version(),
					digest,
				); err != nil {
					return fmt.Errorf("failed to write plugin info: %w", err)
				}
			}
			if err := w.Flush(); err != nil {
				return fmt.Errorf("failed to flush writer: %w", err)
			}

			return nil
		}),
	}

	addCommonFlags(cmd)

	return cmd
}
