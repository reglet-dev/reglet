package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
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

	// Determine operation
	opName := cfg.Operation

	// Legacy config mapping if operation not set but other fields are?
	// The new config requires "operation".
	// If the user uses the old config style { "path": "...", "read_content": true }, we need to map it.
	// But `core.Config` defined "operation" as required.
	// Let's assume strict compliance with new schema or attempt fallback.
	if opName == "" {
		// Fallbacks based on fields presence (best effort)
		if cfg.Contains != "" {
			opName = "content"
		} else if cfg.Checksum != "" {
			opName = "checksum"
		} else if cfg.Permissions != "" {
			opName = "permissions"
		} else {
			opName = "exists"
		}
	}

	handler, ok := core.Plugin.GetHandler("file", opName)
	if !ok {
		return entities.ResultErrorPtr("configuration", fmt.Sprintf("Unknown operation: %s", opName)), nil
	}

	req := &plugin.Request{
		Config: cfg,
		Raw:    configBytes,
	}

	return handler(ctx, req)
}

func init() {
	plugin.Register(&filePlugin{})
}

func main() {}
