package main

import (
	"context"

	"github.com/reglet-dev/reglet-sdk/go/application/config"
	"github.com/reglet-dev/reglet-sdk/go/application/schema"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
	sdknet "github.com/reglet-dev/reglet-sdk/go/net"
)

// httpPlugin implements the sdk.Plugin interface.
type httpPlugin struct {
	client ports.HTTPClient
}

// Describe returns HTTP plugin metadata.
func (p *httpPlugin) Describe(ctx context.Context) (entities.Metadata, error) {
	return entities.Metadata{
		Name:        "http",
		Version:     "1.0.0",
		Description: "HTTP/HTTPS request checking and validation",
		Capabilities: []entities.Capability{
			{
				Category: "network",
				Resource: "outbound:80,443",
			},
		},
	}, nil
}

type HTTPConfig struct {
	URL                  string `json:"url" validate:"required,url" description:"URL to request"`
	Method               string `json:"method" validate:"oneof=GET POST PUT DELETE HEAD OPTIONS PATCH" default:"GET" description:"HTTP method"`
	Body                 string `json:"body,omitempty" description:"Request body"`
	ExpectedStatus       int    `json:"expected_status,omitempty" description:"Expected HTTP status code (optional)"`
	ExpectedBodyContains string `json:"expected_body_contains,omitempty" description:"String that should be present in response body (optional)"`
	BodyPreviewLength    int    `json:"body_preview_length,omitempty" default:"200" description:"Number of characters to include from response body (0 = hash only, -1 = full body)"`
}

// Schema returns config schema.
func (p *httpPlugin) Schema(ctx context.Context) ([]byte, error) {
	return schema.GenerateSchema(HTTPConfig{})
}

// Check executes HTTP request.
func (p *httpPlugin) Check(ctx context.Context, cfgRaw config.Config) (entities.Result, error) {
	var cfg HTTPConfig
	if err := config.Validate(cfgRaw, &cfg); err != nil {
		return entities.ResultError(entities.NewErrorDetail("config", err.Error())), nil
	}
	// Note: RunHTTPCheck expects untyped config.Config, but validation ensures correctness.
	return sdknet.RunHTTPCheck(ctx, cfgRaw, sdknet.WithHTTPClient(p.client))
}
