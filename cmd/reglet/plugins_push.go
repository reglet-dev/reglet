package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/reglet-dev/reglet/internal/domain/entities"
	"github.com/reglet-dev/reglet/internal/domain/values"
	"github.com/spf13/cobra"
)

func init() {
	pluginsCmd.AddCommand(newPluginsPushCmd())
}

func newPluginsPushCmd() *cobra.Command {
	var wasmPath string
	var metaPath string
	var sign bool

	cmd := &cobra.Command{
		Use:   "push <reference>",
		Short: "Push a plugin to a registry",
		Long:  `Push a plugin WASM binary and metadata to an OCI registry.`,
		Example: `  # Push a plugin
  reglet plugins push ghcr.io/my-org/my-plugin:1.0.0 --wasm plugin.wasm --metadata metadata.json

  # Push and sign
  reglet plugins push ghcr.io/my-org/my-plugin:1.0.0 --wasm plugin.wasm --metadata metadata.json --sign`,
		Args: cobra.ExactArgs(1),
		RunE: withContainer(func(ctx *CommandContext, cmd *cobra.Command, args []string) error {
			refStr := args[0]

			if wasmPath == "" || metaPath == "" {
				return fmt.Errorf("both --wasm and --metadata flags are required")
			}

			// Read WASM
			cleanWasmPath := filepath.Clean(wasmPath)
			wasmFile, err := os.Open(cleanWasmPath)
			if err != nil {
				return fmt.Errorf("open wasm file: %w", err)
			}
			defer func() {
				if cerr := wasmFile.Close(); cerr != nil {
					ctx.Logger.Warn("failed to close wasm file", "path", cleanWasmPath, "error", cerr)
				}
			}()

			// Compute digest
			cleanWasmPath = filepath.Clean(wasmPath) // Already cleaned above but good for clarity if blocks move
			wasmBytes, err := os.ReadFile(cleanWasmPath)
			if err != nil {
				return err
			}
			// Compute SHA256 of bytes
			h := sha256.New()
			h.Write(wasmBytes)
			digestStr := fmt.Sprintf("sha256:%x", h.Sum(nil))

			digest, err := values.ParseDigest(digestStr)
			if err != nil {
				return fmt.Errorf("invalid digest: %w", err)
			}

			// Read Metadata
			cleanMetaPath := filepath.Clean(metaPath)
			metaBytes, err := os.ReadFile(cleanMetaPath)
			if err != nil {
				return fmt.Errorf("read metadata: %w", err)
			}
			var metaDto struct {
				Name         string   `json:"name"`
				Version      string   `json:"version"`
				Description  string   `json:"description"`
				Capabilities []string `json:"capabilities"`
			}
			if err := json.Unmarshal(metaBytes, &metaDto); err != nil {
				return fmt.Errorf("parse metadata: %w", err)
			}
			metadata := values.NewPluginMetadata(metaDto.Name, metaDto.Version, metaDto.Description, metaDto.Capabilities)

			// Create Reference
			ref, err := values.ParsePluginReference(refStr)
			if err != nil {
				return fmt.Errorf("invalid reference: %w", err)
			}

			// Create Plugin Entity
			plugin := entities.NewPlugin(ref, digest, metadata)

			// Publish
			ctx.Logger.Info("pushing plugin", "ref", ref.String(), "digest", digest.String())

			// Re-open/Stream WASM
			wasmReader := bytes.NewReader(wasmBytes)

			if err := ctx.Container.PluginService().PublishPlugin(ctx.Context, plugin, wasmReader, sign); err != nil {
				return fmt.Errorf("failed to push plugin: %w", err)
			}

			fmt.Println("Plugin pushed successfully.")
			return nil
		}),
	}

	cmd.Flags().StringVar(&wasmPath, "wasm", "", "Path to WASM binary")
	cmd.Flags().StringVar(&metaPath, "metadata", "", "Path to metadata.json")
	cmd.Flags().BoolVar(&sign, "sign", false, "Sign the plugin")

	addCommonFlags(cmd)

	return cmd
}
