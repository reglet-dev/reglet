package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
)

func init() {
	plugin.Register(&FixturePlugin{})
}

func main() {
	// Empty main for reactor build
}

type FixturePlugin struct{}

func (p *FixturePlugin) Manifest(ctx context.Context) (*entities.Manifest, error) {
	return &entities.Manifest{
		Name:        "fixture",
		Version:     "0.0.1",
		Description: "Test fixture for Reglet Host integration verify",
		Capabilities: []entities.Capability{
			{
				Category: "fs",
				Resource: "/tmp",
				Action:   "read",
			},
			{
				Category: "network",
				Resource: "*:80",
				Action:   "connect",
			},
			{
				Category: "env",
				Resource: "TEST_VAR",
			},
		},
		ConfigSchema: []byte(`{
			"type": "object",
			"properties": {
				"action": { "type": "string" },
				"input": { "type": "string" }
			}
		}`),
	}, nil
}

func (p *FixturePlugin) Check(ctx context.Context, config []byte) (*entities.Result, error) {
	var cfg struct {
		Action string `json:"action"`
		Input  string `json:"input"`
	}
	// Default empty config logic if needed
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return nil, fmt.Errorf("invalid config: %w", err)
		}
	}

	evidence := entities.Result{
		Status: entities.ResultStatusSuccess,
		Data: map[string]interface{}{
			"action": cfg.Action,
			"echo":   cfg.Input,
		},
	}

	// Behave differently based on action to simulate different plugin types if needed
	switch cfg.Action {
	case "fail":
		evidence.Status = entities.ResultStatusFailure
		evidence.Error = &entities.ErrorDetail{Message: "requested failure"}
	case "network_sim":
		evidence.Data["simulated_network"] = true
	case "fs_sim":
		evidence.Data["simulated_fs"] = true
	}

	return &evidence, nil
}
