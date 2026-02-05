package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/reglet-dev/reglet-sdk/application/plugin"
	"github.com/reglet-dev/reglet-sdk/domain/entities"
	"github.com/reglet-dev/reglet-sdk/infrastructure/wasm"
	"github.com/reglet-dev/reglet/plugins/http/core"

	// Import services to trigger auto-registration
	_ "github.com/reglet-dev/reglet/plugins/http/services"
)

type httpPlugin struct{}

func (p *httpPlugin) Manifest(ctx context.Context) (*entities.Manifest, error) {
	return core.Plugin.Manifest(), nil
}

func (p *httpPlugin) Check(ctx context.Context, configBytes []byte) (*entities.Result, error) {
	// Parse config
	var cfgStruct core.HTTPConfig
	if err := json.Unmarshal(configBytes, &cfgStruct); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	cfg := &cfgStruct

	// Determine operation - Always map to "Request"
	handler, ok := core.Plugin.GetHandler("http", "request")
	if !ok {
		return entities.ResultErrorPtr("configuration", "Unknown operation"), nil
	}

	req := &plugin.Request{
		Client: wasm.NewHTTPAdapter(0), // Use default HTTP adapter
		Config: cfg,
		Raw:    configBytes,
	}

	return handler(ctx, req)
}

func init() {
	plugin.Register(&httpPlugin{})
}

func main() {}
