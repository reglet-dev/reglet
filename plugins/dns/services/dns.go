package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
	"github.com/reglet-dev/reglet/plugins/dns/core"
)

// DNSService provides DNS resolution checks.
// Auto-registers on package import via init().
type DNSService struct {
	plugin.Service `name:"dns" desc:"DNS resolution and record validation"`

	Resolve       plugin.Op `desc:"Resolve hostname and return records" method:"ResolveHandler"`
	ValidateA     plugin.Op `desc:"Validate A record matches expected IPs" method:"ValidateAHandler"`
	ValidateMX    plugin.Op `desc:"Validate MX records exist and are correct" method:"ValidateMXHandler"`
	ValidateTXT   plugin.Op `desc:"Validate TXT records contain expected values" method:"ValidateTXTHandler"`
	ValidateCNAME plugin.Op `desc:"Validate CNAME points to expected target" method:"ValidateCNAMEHandler"`
}

func init() {
	plugin.MustRegisterService(core.Plugin, &DNSService{})
}

// Handler implementations

func (s *DNSService) ResolveHandler(ctx context.Context, req *plugin.Request) (*entities.Result, error) {
	cfg := req.Config.(*core.DNSConfig)
	return s.runLookup(ctx, req, cfg.RecordType)
}

func (s *DNSService) ValidateAHandler(ctx context.Context, req *plugin.Request) (*entities.Result, error) {
	return s.runLookup(ctx, req, "A")
}

func (s *DNSService) ValidateMXHandler(ctx context.Context, req *plugin.Request) (*entities.Result, error) {
	return s.runLookup(ctx, req, "MX")
}

func (s *DNSService) ValidateTXTHandler(ctx context.Context, req *plugin.Request) (*entities.Result, error) {
	return s.runLookup(ctx, req, "TXT")
}

func (s *DNSService) ValidateCNAMEHandler(ctx context.Context, req *plugin.Request) (*entities.Result, error) {
	return s.runLookup(ctx, req, "CNAME")
}

// runLookup performs the common DNS lookup and validation logic.
// It dispatches to the correct resolver method based on recordType.
func (s *DNSService) runLookup(ctx context.Context, req *plugin.Request, recordType string) (*entities.Result, error) {
	cfg := req.Config.(*core.DNSConfig)
	resolver := req.Client.(ports.DNSResolver)

	if recordType == "" {
		recordType = "A"
	}

	var vals []string
	var err error

	switch strings.ToUpper(recordType) {
	case "A", "AAAA":
		// LookupHost returns IPs (v4 and v6)
		vals, err = resolver.LookupHost(ctx, cfg.Hostname)
	case "CNAME":
		var cname string
		cname, err = resolver.LookupCNAME(ctx, cfg.Hostname)
		if err == nil {
			vals = []string{cname}
		}
	case "MX":
		var mxs []ports.MXRecord
		mxs, err = resolver.LookupMX(ctx, cfg.Hostname)
		if err == nil {
			for _, mx := range mxs {
				vals = append(vals, fmt.Sprintf("%d %s", mx.Pref, mx.Host))
			}
		}
	case "TXT":
		vals, err = resolver.LookupTXT(ctx, cfg.Hostname)
	case "NS":
		vals, err = resolver.LookupNS(ctx, cfg.Hostname)
	default:
		// Fallback or not supported
		return entities.ResultErrorPtr("configuration", fmt.Sprintf("Unsupported record type: %s", recordType)), nil
	}

	if err != nil {
		return entities.ResultFailurePtr(fmt.Sprintf("DNS lookup failed: %v", err), nil), nil
	}

	data := map[string]any{
		"hostname":    cfg.Hostname,
		"record_type": recordType,
		"records":     vals,
	}

	// Validation
	if len(cfg.ExpectedValues) > 0 {
		missing := []string{}
		for _, expected := range cfg.ExpectedValues {
			found := false
			for _, v := range vals {
				// Exact match or contains for complex records
				if v == expected || strings.Contains(v, expected) {
					found = true
					break
				}
			}
			if !found {
				missing = append(missing, expected)
			}
		}

		if len(missing) > 0 {
			return entities.ResultFailurePtr(
				fmt.Sprintf("Missing expected records: %s", strings.Join(missing, ", ")),
				data,
			), nil
		}
	}

	return entities.ResultSuccessPtr(
		fmt.Sprintf("Resolved %s (%s): %v", cfg.Hostname, recordType, vals),
		data,
	), nil
}
