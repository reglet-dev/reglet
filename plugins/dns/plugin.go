package main

import (
	"context"

	"github.com/reglet-dev/reglet-sdk/go/application/config"
	"github.com/reglet-dev/reglet-sdk/go/application/schema"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
	sdknet "github.com/reglet-dev/reglet-sdk/go/net"
)

// dnsPlugin implements the sdk.Plugin interface.
type dnsPlugin struct {
	resolver ports.DNSResolver
}

// Describe returns plugin metadata.
func (p *dnsPlugin) Describe(ctx context.Context) (entities.Metadata, error) {
	return entities.Metadata{
		Name:        "dns",
		Version:     "1.0.0",
		Description: "DNS resolution and record validation",
		Capabilities: []entities.Capability{
			{
				Category: "network",
				Resource: "outbound:53", // Required for DNS lookups
			},
		},
	}, nil
}

type DNSConfig struct {
	Hostname   string `json:"hostname" validate:"required" description:"Hostname to resolve"`
	RecordType string `json:"record_type" validate:"oneof=A AAAA CNAME MX TXT NS" default:"A" description:"DNS record type to query"`
	Nameserver string `json:"nameserver,omitempty" description:"Custom nameserver (optional, e.g., 8.8.8.8:53)"`
}

// Schema returns the JSON schema for the plugin's configuration.
func (p *dnsPlugin) Schema(ctx context.Context) ([]byte, error) {
	return schema.GenerateSchema(DNSConfig{})
}

// Check executes the DNS observation.
func (p *dnsPlugin) Check(ctx context.Context, cfgRaw config.Config) (entities.Result, error) {
	var cfg DNSConfig
	if err := config.Validate(cfgRaw, &cfg); err != nil {
		return entities.ResultError(entities.NewErrorDetail("config", err.Error())), nil
	}
	return sdknet.RunDNSCheck(ctx, cfgRaw, sdknet.WithDNSResolver(p.resolver))
}
