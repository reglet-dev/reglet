package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
	"github.com/reglet-dev/reglet-sdk/go/infrastructure/wasm"
	"github.com/reglet-dev/reglet/plugins/http/core"

	// Import services to trigger auto-registration
	_ "github.com/reglet-dev/reglet/plugins/http/services"
)

type httpPlugin struct {
	client ports.HTTPClient
}

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

	opName := "check_status" // Default
	method := strings.ToUpper(cfg.Method)
	switch method {
	case "GET":
		opName = "get"
	case "POST":
		opName = "post"
	case "HEAD":
		opName = "head"
	}

	handler, ok := core.Plugin.GetHandler("http", opName)
	if !ok {
		return entities.ResultErrorPtr("configuration",
			fmt.Sprintf("Unknown operation for method: %s", method)), nil
	}

	// Use the provided client or a default one (though in WASM we rely on imports)
	client := p.client
	if client == nil {
		// In WASM, we should use the SDK's default client which wraps host functions
		client = wasm.NewHTTPAdapter(0)
	}

	req := &plugin.Request{
		Client: client,
		Config: cfg,
		Raw:    configBytes,
	}

	return handler(ctx, req)
}

func main() {
	plugin.Register(&httpPlugin{
		client: wasm.NewHTTPAdapter(0),
	})
}
