package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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

	res, err := handler(ctx, req)
	if err != nil {
		return nil, err
	}

	// Validation
	if res.Status == entities.ResultStatusSuccess && res.Data != nil {
		// Exit code check
		actualExit := 0
		if val, ok := res.Data["exit_code"]; ok {
			switch v := val.(type) {
			case int:
				actualExit = v
			case float64:
				actualExit = int(v)
			}
		}

		if actualExit != cfg.ExpectedExit {
			res.Status = entities.ResultStatusFailure
			res.Error = &entities.ErrorDetail{
				Message: fmt.Sprintf("Exit code mismatch: expected %d, got %d", cfg.ExpectedExit, actualExit),
				Type:    "validation",
			}
		}

		// Output check
		if cfg.ExpectedOutput != "" {
			stdout, _ := res.Data["stdout"].(string)
			if !strings.Contains(stdout, cfg.ExpectedOutput) {
				res.Status = entities.ResultStatusFailure
				res.Error = &entities.ErrorDetail{
					Message: fmt.Sprintf("Output mismatch: expected '%s' in stdout", cfg.ExpectedOutput),
					Type:    "validation",
				}
			}
		}
	}

	return res, nil
}

func init() {
	plugin.Register(&commandPlugin{})
}

func main() {}
