package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/reglet-dev/reglet-sdk/application/plugin"
	"github.com/reglet-dev/reglet-sdk/domain/entities"
	"github.com/reglet-dev/reglet-sdk/infrastructure/wasm"
	"github.com/reglet-dev/reglet/plugins/smtp/core"

	// Import services to trigger auto-registration
	_ "github.com/reglet-dev/reglet/plugins/smtp/services"
)

type smtpPlugin struct{}

func (p *smtpPlugin) Manifest(ctx context.Context) (*entities.Manifest, error) {
	return core.Plugin.Manifest(), nil
}

func (p *smtpPlugin) Check(ctx context.Context, configBytes []byte) (*entities.Result, error) {
	// Parse config
	var cfgStruct core.SMTPConfig
	if err := json.Unmarshal(configBytes, &cfgStruct); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	cfg := &cfgStruct

	// Determine operation
	handler, ok := core.Plugin.GetHandler("smtp", "connect")
	if !ok {
		return entities.ResultErrorPtr("configuration", "Unknown operation"), nil
	}

	req := &plugin.Request{
		Client: wasm.NewSMTPAdapter(),
		Config: cfg,
		Raw:    configBytes,
	}

	return handler(ctx, req)
}

func init() {
	plugin.Register(&smtpPlugin{})
}

func main() {}
