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
	"github.com/reglet-dev/reglet/plugins/dns/core"

	// Import services to trigger auto-registration
	_ "github.com/reglet-dev/reglet/plugins/dns/services"
)

type dnsPlugin struct {
	resolver ports.DNSResolver
}

func (p *dnsPlugin) Manifest(ctx context.Context) (*entities.Manifest, error) {
	return core.Plugin.Manifest(), nil
}

func (p *dnsPlugin) Check(ctx context.Context, configBytes []byte) (*entities.Result, error) {
	// Parse config
	var cfgStruct core.DNSConfig
	if err := json.Unmarshal(configBytes, &cfgStruct); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	cfg := &cfgStruct

	// Determine operation
	opName := "resolve" // Default
	// We could map Validate* ops based on intent, but usually "resolve" handles all with config
	// However, if we want to use specific ops:
	if len(cfg.ExpectedValues) > 0 {
		switch strings.ToUpper(cfg.RecordType) {
		case "A":
			opName = "validate_a"
		case "MX":
			opName = "validate_mx"
		case "TXT":
			opName = "validate_txt"
		case "CNAME":
			opName = "validate_cname"
		}
	}

	handler, ok := core.Plugin.GetHandler("dns", opName)
	if !ok {
		// Fallback to resolve if specific validate op not found (shouldn't happen if defined)
		handler, ok = core.Plugin.GetHandler("dns", "resolve")
		if !ok {
			return entities.ResultErrorPtr("configuration", "Unknown operation"), nil
		}
	}

	// Use provided resolver or default WASM adapter
	resolver := p.resolver
	if resolver == nil {
		resolver = wasm.NewDNSAdapter("", 0)
	}

	req := &plugin.Request{
		Client: resolver,
		Config: cfg,
		Raw:    configBytes,
	}

	return handler(ctx, req)
}

func main() {
	plugin.Register(&dnsPlugin{
		resolver: wasm.NewDNSAdapter("", 0),
	})
}
