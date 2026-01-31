package main

import (
	"context"

	"github.com/reglet-dev/reglet-sdk/go/application/config"
	"github.com/reglet-dev/reglet-sdk/go/application/schema"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet-sdk/go/domain/ports"

	"github.com/reglet-dev/reglet/plugins/aws/core"
)

// awsPlugin implements the SDK Plugin interface.
type awsPlugin struct {
	// httpClient is injectable for testing
	httpClient ports.HTTPClient
}

// Describe returns plugin metadata.
func (p *awsPlugin) Describe(ctx context.Context) (entities.Metadata, error) {
	return entities.Metadata{
		Name:        "aws",
		Version:     "1.0.0",
		Description: "AWS infrastructure compliance and security validation",
		Capabilities: []entities.Capability{
			{Category: "network:outbound", Resource: "*.amazonaws.com:443"},
			{Category: "environment", Resource: "AWS_ACCESS_KEY_ID"},
			{Category: "environment", Resource: "AWS_SECRET_ACCESS_KEY"},
			{Category: "environment", Resource: "AWS_SESSION_TOKEN"},
			{Category: "environment", Resource: "AWS_REGION"},
			{Category: "environment", Resource: "AWS_DEFAULT_REGION"},
		},
	}, nil
}

// Schema returns the JSON schema for plugin configuration.
func (p *awsPlugin) Schema(ctx context.Context) ([]byte, error) {
	return schema.GenerateSchema(core.AWSConfig{})
}

// Check executes an AWS compliance check.
func (p *awsPlugin) Check(ctx context.Context, rawConfig map[string]any) (entities.Result, error) {
	// Parse configuration
	cfg, err := parseConfig(rawConfig)
	if err != nil {
		return entities.ResultError(&entities.ErrorDetail{ //nolint:nilerr
			Type:    "config",
			Message: err.Error(),
		}), nil
	}

	// Load credentials
	creds, err := core.GetCredentials(cfg)
	if err != nil {
		return entities.ResultError(&entities.ErrorDetail{ //nolint:nilerr
			Type:    "auth",
			Message: err.Error(),
		}), nil
	}

	// Create AWS client
	client := core.NewAWSClient(creds, cfg.TimeoutSeconds)
	if p.httpClient != nil {
		client.HTTPClient = p.httpClient
	}

	// Route to service handler
	result, err := core.RouteToService(ctx, client, cfg)
	if err != nil {
		return entities.ResultError(core.MapAWSErrorToSDK(err)), nil
	}

	return result, nil
}

// parseConfig extracts and validates the AWSConfig from raw config.
func parseConfig(rawConfig map[string]any) (*core.AWSConfig, error) {
	cfg := &core.AWSConfig{}

	// Required fields
	var err error
	cfg.Service, err = config.MustGetString(rawConfig, "service")
	if err != nil {
		return nil, err
	}

	cfg.Operation, err = config.MustGetString(rawConfig, "operation")
	if err != nil {
		return nil, err
	}

	// Optional fields with defaults
	cfg.Region = config.GetStringDefault(rawConfig, "region", "")
	cfg.TimeoutSeconds = config.GetIntDefault(rawConfig, "timeout_seconds", 30)

	// Filters (optional)
	if filters, ok := rawConfig["filters"]; ok {
		if filtersMap, ok := filters.(map[string]any); ok {
			cfg.Filters = make(map[string][]string)
			for k, v := range filtersMap {
				if values, ok := v.([]any); ok {
					for _, val := range values {
						if s, ok := val.(string); ok {
							cfg.Filters[k] = append(cfg.Filters[k], s)
						}
					}
				}
			}
		}
	}

	return cfg, nil
}
