package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet-sdk/go/infrastructure/wasm"
	"github.com/reglet-dev/reglet/plugins/command/core"

	// Import services to trigger auto-registration
	_ "github.com/reglet-dev/reglet/plugins/command/services"
)

type commandPlugin struct{}

func (p *commandPlugin) Manifest(ctx context.Context) (*entities.Manifest, error) {
	return core.Plugin.Manifest(), nil
}

func (p *commandPlugin) Check(ctx context.Context, configBytes []byte) (*entities.Result, error) {
	// Parse config
	var cfgStruct core.CommandConfig
	if err := json.Unmarshal(configBytes, &cfgStruct); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	cfg := &cfgStruct

	// Maps to single operation "Execute"
	handler, ok := core.Plugin.GetHandler("execution", "execute")
	if !ok {
		return entities.ResultErrorPtr("configuration", "Unknown operation"), nil
	}

	req := &plugin.Request{
		Client: wasm.NewExecAdapter(),
		Config: cfg,
		Raw:    configBytes,
	}

	return handler(ctx, req)
}

func init() {
	plugin.Register(&commandPlugin{})
}

func main() {}
