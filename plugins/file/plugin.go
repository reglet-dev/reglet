package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	regletsdk "github.com/whiskeyjimbo/reglet/sdk"
)

// filePlugin implements the sdk.Plugin interface.
type filePlugin struct{}

// Describe returns plugin metadata.
func (p *filePlugin) Describe(ctx context.Context) (regletsdk.Metadata, error) {
	return regletsdk.Metadata{
		Name:        "file",
		Version:     "1.0.0",
		Description: "File existence and content checks",
		Capabilities: []regletsdk.Capability{
			{
				Kind:    "fs",
				Pattern: "read:**", // Simple pattern, host enforces actual paths
			},
		},
	}, nil
}

type FileConfig struct {
	Path string `json:"path" validate:"required" description:"Path to file to check"`
	Mode string `json:"mode" validate:"oneof=exists readable content" default:"exists" description:"Check mode: exists, readable, or content"`
}

// Schema returns the JSON schema for the plugin's configuration.
func (p *filePlugin) Schema(ctx context.Context) ([]byte, error) {
	return regletsdk.GenerateSchema(FileConfig{})
}

// Check executes the file observation.
func (p *filePlugin) Check(ctx context.Context, config regletsdk.Config) (regletsdk.Evidence, error) {
	// Set default mode if missing
	if _, ok := config["mode"]; !ok {
		config["mode"] = "exists"
	}

	var cfg FileConfig
	if err := regletsdk.ValidateConfig(config, &cfg); err != nil {
		return regletsdk.ConfigError(err), nil
	}

	var result map[string]interface{}

	switch cfg.Mode {
	case "exists":
		_, err := os.Stat(cfg.Path)
		if err != nil {
			// File doesn't exist - this is a check failure
			return regletsdk.Failure("fs", fmt.Sprintf("file not found: %v", err)), nil
		}
		// File exists - check passes
		result = map[string]interface{}{
			"exists": true,
		}

	case "readable":
		file, err := os.Open(cfg.Path)
		if err == nil {
			file.Close()
			result = map[string]interface{}{
				"readable": true,
			}
		} else {
			return regletsdk.Failure("fs", fmt.Sprintf("file not readable: %v", err)), nil
		}

	case "content":
		content, err := os.ReadFile(cfg.Path)
		if err == nil {
			result = map[string]interface{}{
				"content_b64": base64.StdEncoding.EncodeToString(content),
				"encoding":    "base64",
				"size":        len(content),
			}
		} else {
			return regletsdk.Failure("fs", fmt.Sprintf("failed to read file: %v", err)), nil
		}
	}

	// Add common fields
	result["path"] = cfg.Path
	result["mode"] = cfg.Mode

	return regletsdk.Success(result), nil
}
