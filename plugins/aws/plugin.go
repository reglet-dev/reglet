package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet/plugins/aws/core"

	// Import services to trigger auto-registration
	_ "github.com/reglet-dev/reglet/plugins/aws/services"
)

type awsPlugin struct{}

func (p *awsPlugin) Manifest(ctx context.Context) (*entities.Manifest, error) {
	return core.Plugin.Manifest(), nil
}

func (p *awsPlugin) Check(ctx context.Context, configBytes []byte) (*entities.Result, error) {
	// Parse config
	var cfgStruct core.AWSConfig
	if err := json.Unmarshal(configBytes, &cfgStruct); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	cfg := &cfgStruct

	// Get credentials and create client
	creds, err := core.GetCredentials(cfg)
	if err != nil {
		return nil, err
	}
	client := core.NewAWSClient(creds, cfg.TimeoutSeconds)

	// Get handler from registry
	handler, ok := core.Plugin.GetHandler(cfg.Service, cfg.Operation)
	if !ok {
		return entities.ResultErrorPtr("configuration",
			fmt.Sprintf("Unknown operation: %s/%s", cfg.Service, cfg.Operation)), nil
	}

	// Build request and execute
	req := &plugin.Request{
		Client: client,
		Config: cfg,
		Raw:    configBytes,
	}

	return handler(ctx, req)
}
