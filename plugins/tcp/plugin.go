package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet-sdk/go/infrastructure/wasm"
	"github.com/reglet-dev/reglet/plugins/tcp/core"

	// Import services to trigger auto-registration
	_ "github.com/reglet-dev/reglet/plugins/tcp/services"
)

type tcpPlugin struct{}

func (p *tcpPlugin) Manifest(ctx context.Context) (*entities.Manifest, error) {
	return core.Plugin.Manifest(), nil
}

func (p *tcpPlugin) Check(ctx context.Context, configBytes []byte) (*entities.Result, error) {
	// Parse config
	var cfgStruct core.TCPConfig

	// Pre-processing for loose types logic?
	// If the user provided Port as a string in JSON, standard unmarshal to int will fail.
	// We can try to repair it or just let it fail.
	// For legacy compatibility, we might need a distinct unmarshal step or use a custom type.
	// However, `core.Config` defined Port as `int`.
	// Let's rely on standard JSON behavior. If incompatible, error.

	if err := json.Unmarshal(configBytes, &cfgStruct); err != nil {
		// Attempt fallback if error is about string->int conversion?
		// Too complex. Let's assume correct config for now.
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	cfg := &cfgStruct

	// Determine operation
	handler, ok := core.Plugin.GetHandler("tcp", "connect")
	if !ok {
		return entities.ResultErrorPtr("configuration", "Unknown operation"), nil
	}

	// Create client with default adapter
	// Note: We need NewTCPAdapter. Checking SDK...
	// SDK should provide `wasm.NewTCPAdapter()`.

	req := &plugin.Request{
		Client: wasm.NewTCPAdapter(),
		Config: cfg,
		Raw:    configBytes,
	}

	return handler(ctx, req)
}

func init() {
	plugin.Register(&tcpPlugin{})
}

func main() {}
