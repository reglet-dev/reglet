package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/reglet-dev/reglet-sdk/application/plugin"
	"github.com/reglet-dev/reglet-sdk/domain/entities"
	"github.com/reglet-dev/reglet/plugins/file/core"

	// Import services to trigger auto-registration
	_ "github.com/reglet-dev/reglet/plugins/file/services"
)

type filePlugin struct{}

func (p *filePlugin) Manifest(ctx context.Context) (*entities.Manifest, error) {
	return core.Plugin.Manifest(), nil
}

func (p *filePlugin) Check(ctx context.Context, configBytes []byte) (*entities.Result, error) {
	// Parse config
	var cfgStruct core.FileConfig
	if err := json.Unmarshal(configBytes, &cfgStruct); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	cfg := &cfgStruct

	// Determine operation (fallback logic for legacy compatibility)
	if cfg.Operation == "" {
		if cfg.Contains != "" {
			cfg.Operation = "content"
		} else if cfg.Checksum != "" {
			cfg.Operation = "checksum"
		} else if cfg.Permissions != "" {
			cfg.Operation = "permissions"
		} else {
			cfg.Operation = "exists"
		}
	}

	// Always use "Check" operation
	handler, ok := core.Plugin.GetHandler("file", "check")
	if !ok {
		return entities.ResultErrorPtr("configuration", "Unknown operation"), nil
	}

	newConfigBytes, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal adjusted config: %w", err)
	}

	req := &plugin.Request{
		Config: cfg,
		Raw:    newConfigBytes,
	}

	return handler(ctx, req)
}

func init() {
	plugin.Register(&filePlugin{})
}

func main() {}
