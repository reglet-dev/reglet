package main

import (
	"context"
	"fmt"
	"net"
	"time"

	regletsdk "github.com/whiskeyjimbo/reglet/sdk"
)

// dnsResolver interface abstracts the network calls for testability.
// This allows us to mock the SDK's networking functions in tests.
type dnsResolver interface {
	LookupHost(ctx context.Context, host string, nameserver string) ([]string, error)
	LookupCNAME(ctx context.Context, host string, nameserver string) (string, error)
	LookupMX(ctx context.Context, host string, nameserver string) ([]string, error)
	LookupTXT(ctx context.Context, host string, nameserver string) ([]string, error)
	LookupNS(ctx context.Context, host string, nameserver string) ([]string, error)
}

// dnsPlugin implements the sdk.Plugin interface.
type dnsPlugin struct {
	resolver dnsResolver
}

// Describe returns plugin metadata.
func (p *dnsPlugin) Describe(ctx context.Context) (regletsdk.Metadata, error) {
	return regletsdk.Metadata{
		Name:        "dns",
		Version:     "1.0.0",
		Description: "DNS resolution and record validation",
		Capabilities: []regletsdk.Capability{
			{
				Kind:    "network",
				Pattern: "outbound:53", // Required for DNS lookups
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
	return regletsdk.GenerateSchema(DNSConfig{})
}

// Check executes the DNS observation.
func (p *dnsPlugin) Check(ctx context.Context, config regletsdk.Config) (regletsdk.Evidence, error) {
	// Set default record type if not provided
	// This must be done BEFORE validation because ValidateConfig checks "oneof" on the struct,
	// and if missing in the map, it becomes empty string in struct which fails validation.
	if _, ok := config["record_type"]; !ok {
		config["record_type"] = "A"
	}

	var cfg DNSConfig
	if err := regletsdk.ValidateConfig(config, &cfg); err != nil {
		return regletsdk.ConfigError(err), nil
	}

	// Perform DNS lookup using the injected resolver
	start := time.Now()
	records, err := p.performDNSLookup(ctx, cfg.Hostname, cfg.RecordType, cfg.Nameserver)
	queryTime := time.Since(start).Milliseconds()

	if err != nil {
		return regletsdk.NetworkError(fmt.Sprintf("DNS lookup failed: %v", err), err), nil
	}

	// Return success evidence
	return regletsdk.Success(map[string]interface{}{
		"hostname":      cfg.Hostname,
		"record_type":   cfg.RecordType,
		"records":       records,
		"record_count":  len(records),
		"query_time_ms": queryTime,
	}), nil
}

// performDNSLookup executes the actual DNS lookup using the injected resolver.
func (p *dnsPlugin) performDNSLookup(ctx context.Context, hostname, recordType, nameserver string) ([]string, error) {
	switch recordType {
	case "A":
		ips, err := p.resolver.LookupHost(ctx, hostname, nameserver)
		if err != nil {
			return nil, err
		}
		var ipv4s []string
		for _, ip := range ips {
			parsed := net.ParseIP(ip)
			if parsed != nil && parsed.To4() != nil {
				ipv4s = append(ipv4s, ip)
			}
		}
		return ipv4s, nil

	case "AAAA":
		ips, err := p.resolver.LookupHost(ctx, hostname, nameserver)
		if err != nil {
			return nil, err
		}
		var ipv6s []string
		for _, ip := range ips {
			parsed := net.ParseIP(ip)
			if parsed != nil && parsed.To4() == nil {
				ipv6s = append(ipv6s, ip)
			}
		}
		return ipv6s, nil

	case "CNAME":
		cname, err := p.resolver.LookupCNAME(ctx, hostname, nameserver)
		if err != nil {
			return nil, err
		}
		return []string{cname}, nil

	case "MX":
		return p.resolver.LookupMX(ctx, hostname, nameserver)

	case "TXT":
		return p.resolver.LookupTXT(ctx, hostname, nameserver)

	case "NS":
		return p.resolver.LookupNS(ctx, hostname, nameserver)

	default:
		return nil, fmt.Errorf("unsupported record type: %s", recordType)
	}
}