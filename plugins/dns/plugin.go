//go:build wasip1

package main

import (
	"context"
	"errors"
	"time"

	regletsdk "github.com/whiskeyjimbo/reglet/sdk"
	regletnet "github.com/whiskeyjimbo/reglet/sdk/net"
	"github.com/whiskeyjimbo/reglet/wireformat"
)

// dnsPlugin implements the sdk.Plugin interface.
type dnsPlugin struct{}

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
		},
		nil
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
		return regletsdk.Evidence{
			Status: false,
			Error:  regletsdk.ToErrorDetail(&regletsdk.ConfigError{Err: err}),
		}, nil
	}

	start := time.Now()
	resolver := &regletnet.WasmResolver{Nameserver: cfg.Nameserver}
	dnsResponseWire, sdkErr := resolver.Lookup(ctx, cfg.Hostname, cfg.RecordType) // sdkErr is *wireformat.ErrorDetail or other Go error type
	queryTime := time.Since(start).Milliseconds()

	// Prepare data for evidence.
	data := map[string]interface{}{
		"hostname":      cfg.Hostname,
		"record_type":   cfg.RecordType,
		"query_time_ms": queryTime,
	}

	var evidence regletsdk.Evidence
	var finalErrorDetail *wireformat.ErrorDetail

	if sdkErr != nil {
		// If SDK returned a Go error, it signifies a problem with the Host call or its processing.
		// sdkErr is *wireformat.ErrorDetail (due to SDK's LookupRaw function mapping it).
		if errors.As(sdkErr, &finalErrorDetail) {
			if finalErrorDetail.Type == "config" {
				evidence = regletsdk.Evidence{
					Status: false,
					Error:  regletsdk.ToErrorDetail(&regletsdk.ConfigError{Err: finalErrorDetail}),
				}
			} else {
				evidence = regletsdk.Evidence{
					Status: false,
					Error: regletsdk.ToErrorDetail(&regletsdk.NetworkError{
						Operation: "dns_lookup",
						Target:    cfg.Hostname,
						Err:       finalErrorDetail,
					}),
				}
			}
		} else {
			// Generic Go error from SDK, not specific wireformat error.
			finalErrorDetail = &wireformat.ErrorDetail{
				Message: sdkErr.Error(),
				Type:    "internal",
			}
			evidence = regletsdk.Failure("dns_sdk_error", finalErrorDetail.Message)
		}
	} else if dnsResponseWire.Error != nil {
		// Host returned a structured error in the wire response (e.g. DNS NXDOMAIN, timeout)
		finalErrorDetail = dnsResponseWire.Error
		if finalErrorDetail.Type == "config" {
			evidence = regletsdk.Evidence{
				Status: false,
				Error:  regletsdk.ToErrorDetail(&regletsdk.ConfigError{Err: finalErrorDetail}),
			}
		} else {
			evidence = regletsdk.Evidence{
				Status: false,
				Error: regletsdk.ToErrorDetail(&regletsdk.NetworkError{
					Operation: "dns_lookup",
					Target:    cfg.Hostname,
					Err:       finalErrorDetail,
				}),
			}
		}
	} else {
		// Success path: host returned no error, populate records
		recordCount := 0
		if dnsResponseWire.Records != nil {
			data["records"] = dnsResponseWire.Records
			recordCount = len(dnsResponseWire.Records)
		}
		if dnsResponseWire.MXRecords != nil {
			var mxRecords []map[string]interface{}
			for _, mx := range dnsResponseWire.MXRecords {
				mxRecords = append(mxRecords, map[string]interface{}{"host": mx.Host, "pref": mx.Pref})
			}
			data["mx_records"] = mxRecords
			recordCount = len(mxRecords)
		}
		data["record_count"] = recordCount
		evidence = regletsdk.Success(data) // Final success
	}

	// Always populate error flags and message into Evidence.Data for consistent OPA policy access.
	if finalErrorDetail != nil {
		data["error_message"] = finalErrorDetail.Message
		data["is_timeout"] = finalErrorDetail.IsTimeout
		data["is_not_found"] = finalErrorDetail.IsNotFound
	} else {
		data["is_timeout"] = false
		data["is_not_found"] = false
	}
	evidence.Data = data

	return evidence, nil
}
