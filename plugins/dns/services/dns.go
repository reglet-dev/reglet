package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
	"github.com/reglet-dev/reglet/plugins/dns/core"
)

// DNSService provides DNS resolution operations.
type DNSService struct {
	plugin.Service `name:"dns" desc:"DNS resolution and record lookup"`

	// Resolve looks up DNS records for a hostname
	Resolve plugin.Op[ResolveInput, ResolveOutput] `desc:"Resolve hostname and return DNS records" method:"ResolveHandler"`
}

func init() {
	// Register operation types with examples
	plugin.RegisterOp[ResolveInput, ResolveOutput]("Resolve",
		plugin.Example[ResolveInput, ResolveOutput]{
			Name:        "basic_a",
			Description: "Resolve A record for a hostname",
			Input: ResolveInput{
				Hostname:   "example.com",
				RecordType: "A",
			},
			ExpectedOutput: &ResolveOutput{
				Hostname:   "example.com",
				RecordType: "A",
				Records:    []string{"93.184.216.34"},
			},
		},
		plugin.Example[ResolveInput, ResolveOutput]{
			Name:        "mx_records",
			Description: "Resolve MX records for email routing",
			Input: ResolveInput{
				Hostname:   "example.com",
				RecordType: "MX",
			},
		},
		plugin.Example[ResolveInput, ResolveOutput]{
			Name:        "txt_records",
			Description: "Resolve TXT records for SPF/DKIM",
			Input: ResolveInput{
				Hostname:   "example.com",
				RecordType: "TXT",
			},
		},
		plugin.Example[ResolveInput, ResolveOutput]{
			Name:          "nxdomain",
			Description:   "Error case: non-existent domain",
			Input:         ResolveInput{Hostname: "this-domain-does-not-exist.invalid"},
			ExpectedError: "DNS lookup failed",
		},
	)

	// Register the service
	plugin.MustRegisterService(core.Plugin, &DNSService{})
}

// ResolveHandler performs DNS resolution.
func (s *DNSService) ResolveHandler(ctx context.Context, in *ResolveInput) (*ResolveOutput, error) {
	// Get DNS resolver from context
	resolver := plugin.GetClient[ports.DNSResolver](ctx)

	recordType := in.RecordType
	if recordType == "" {
		recordType = "A"
	}

	var records []string
	var err error

	switch strings.ToUpper(recordType) {
	case "A", "AAAA":
		records, err = resolver.LookupHost(ctx, in.Hostname)
	case "MX":
		mxs, e := resolver.LookupMX(ctx, in.Hostname)
		err = e
		if err == nil {
			for _, mx := range mxs {
				records = append(records, fmt.Sprintf("%d %s", mx.Pref, mx.Host))
			}
		}
	case "TXT":
		records, err = resolver.LookupTXT(ctx, in.Hostname)
	case "CNAME":
		cname, e := resolver.LookupCNAME(ctx, in.Hostname)
		err = e
		if err == nil {
			records = []string{cname}
		}
	case "NS":
		records, err = resolver.LookupNS(ctx, in.Hostname)
	default:
		return nil, fmt.Errorf("unsupported record type: %s", recordType)
	}

	if err != nil {
		return nil, fmt.Errorf("DNS lookup failed: %w", err)
	}

	return &ResolveOutput{
		Hostname:    in.Hostname,
		RecordType:  recordType,
		Records:     records,
		Nameserver:  in.Nameserver,
		RecordCount: len(records),
	}, nil
}
