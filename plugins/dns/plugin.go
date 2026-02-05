package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/reglet-dev/reglet-sdk/application/plugin"
	"github.com/reglet-dev/reglet-sdk/domain/entities"
	"github.com/reglet-dev/reglet-sdk/infrastructure/wasm"
	"github.com/reglet-dev/reglet/plugins/dns/core"

	// Import services to trigger auto-registration
	_ "github.com/reglet-dev/reglet/plugins/dns/services"
)

type dnsPlugin struct{}

func (p *dnsPlugin) Manifest(ctx context.Context) (*entities.Manifest, error) {
	return core.Plugin.Manifest(), nil
}

func (p *dnsPlugin) Check(ctx context.Context, configBytes []byte) (*entities.Result, error) {
	handler, ok := core.Plugin.GetHandler("dns", "resolve")
	if !ok {
		return entities.ResultErrorPtr("configuration", "handler not found"), nil
	}

	var cfg struct {
		RecordType string `json:"record_type"`
	}
	if err := json.Unmarshal(configBytes, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	req := &plugin.Request{
		Client: wasm.NewDNSAdapter("", 0),
		Raw:    configBytes,
	}

	return handler(ctx, req)
}

func init() {
	plugin.Register(&dnsPlugin{})
}

func main() {}
