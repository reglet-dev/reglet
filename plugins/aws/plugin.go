//go:build wasip1

package main

import (
	"context"
	"net/http"

	regletsdk "github.com/whiskeyjimbo/reglet/sdk"
	regletnet "github.com/whiskeyjimbo/reglet/sdk/net"
)

// awsPlugin implements the sdk.Plugin interface.
type awsPlugin struct {
	// client is an optional HTTP client for testing purposes.
	// If nil, the SDK's default WASM transport is used.
	client *http.Client
}

// Describe returns AWS plugin metadata.
func (p *awsPlugin) Describe(ctx context.Context) (regletsdk.Metadata, error) {
	return regletsdk.Metadata{
		Name:        "aws",
		Version:     "1.0.0",
		Description: "AWS infrastructure compliance and security validation",
		Capabilities: []regletsdk.Capability{
			{Kind: "network", Pattern: "outbound:443"}, // HTTPS to AWS APIs
			{Kind: "env", Pattern: "AWS_*"},            // AWS credentials
		},
	}, nil
}

// Schema returns config schema.
func (p *awsPlugin) Schema(ctx context.Context) ([]byte, error) {
	return regletsdk.GenerateSchema(AWSConfig{})
}

// Check executes AWS API request.
func (p *awsPlugin) Check(ctx context.Context, config regletsdk.Config) (regletsdk.Evidence, error) {
	var cfg AWSConfig
	if err := regletsdk.ValidateConfig(config, &cfg); err != nil {
		return regletsdk.Evidence{
			Status: false,
			Error:  regletsdk.ToErrorDetail(&regletsdk.ConfigError{Err: err}),
		}, nil
	}

	// Initialize HTTP client if not already set
	if p.client == nil {
		p.client = &http.Client{
			Transport: &regletnet.WasmTransport{},
		}
	}

	return p.handleService(ctx, cfg)
}