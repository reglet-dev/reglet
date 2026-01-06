package hostfuncs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/tetratelabs/wazero/api"
)

// DNSLookupResult is an intermediate struct to hold the DNS lookup results before converting to wire format.
type DNSLookupResult struct {
	Records   []string
	MXRecords []MXRecordWire
}

// DNSLookup performs DNS resolution on behalf of the plugin.
// It receives a packed uint64 (ptr+len) pointing to a JSON-encoded DNSRequestWire.
// It returns a packed uint64 (ptr+len) pointing to a JSON-encoded DNSResponseWire.
func DNSLookup(ctx context.Context, mod api.Module, stack []uint64, checker *CapabilityChecker) {
	// Stack contains a single uint64 which is packed ptr+len of the request.
	requestPacked := stack[0]
	ptr, length := unpackPtrLen(requestPacked)

	requestBytes, ok := mod.Memory().Read(ptr, length)
	if !ok {
		// This is a critical error, Host could not read Guest memory.
		errMsg := "hostfuncs: failed to read DNS request from Guest memory"
		slog.ErrorContext(ctx, errMsg)
		stack[0] = hostWriteResponse(ctx, mod, DNSResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "internal"},
		})
		return
	}

	var request DNSRequestWire
	if err := json.Unmarshal(requestBytes, &request); err != nil {
		errMsg := fmt.Sprintf("hostfuncs: failed to unmarshal DNS request: %v", err)
		slog.ErrorContext(ctx, errMsg)
		stack[0] = hostWriteResponse(ctx, mod, DNSResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "internal"},
		})
		return
	}

	// Create a new context from the wire format, with parent ctx for cancellation.
	lookupCtx, cancel := createContextFromWire(ctx, request.Context)
	defer cancel() // Ensure context resources are released.

	// 1. Check capability
	pluginName := mod.Name()
	if name, ok := PluginNameFromContext(ctx); ok {
		pluginName = name
	}

	if err := checker.Check(pluginName, "network", "outbound:53"); err != nil {
		errMsg := fmt.Sprintf("permission denied: %v", err)
		slog.WarnContext(ctx, errMsg, "hostname", request.Hostname)
		stack[0] = hostWriteResponse(ctx, mod, DNSResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "capability"},
		})
		return
	}

	// 2. Validate input
	if request.Hostname == "" {
		errMsg := "hostname cannot be empty"
		slog.WarnContext(ctx, errMsg)
		stack[0] = hostWriteResponse(ctx, mod, DNSResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "config"},
		})
		return
	}

	// 3. Perform DNS lookup
	dnsResult, err := performDNSLookup(lookupCtx, request.Hostname, request.Type, request.Nameserver)
	if err != nil {
		errMsg := fmt.Sprintf("DNS lookup failed: %v", err)
		slog.ErrorContext(ctx, errMsg, "hostname", request.Hostname, "record_type", request.Type)
		stack[0] = hostWriteResponse(ctx, mod, DNSResponseWire{
			Error: toErrorDetail(err),
		})
		return
	}

	// 4. Write success response
	stack[0] = hostWriteResponse(ctx, mod, DNSResponseWire{
		Records:   dnsResult.Records,
		MXRecords: dnsResult.MXRecords,
	})
}

// performDNSLookup executes the actual DNS lookup based on record type
func performDNSLookup(ctx context.Context, hostname string, recordType string, nameserver string) (*DNSLookupResult, error) {
	var resolver *net.Resolver

	if nameserver != "" {
		// Use custom resolver
		resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, _, _ string) (net.Conn, error) { // network/address unused: always UDP to nameserver
				d := net.Dialer{
					Timeout: 5 * time.Second, // Default timeout for connection
				}
				return d.DialContext(ctx, "udp", nameserver)
			},
		}
	} else {
		// Use default resolver
		resolver = net.DefaultResolver
	}

	result := &DNSLookupResult{}

	switch recordType {
	case "A":
		ips, err := resolver.LookupHost(ctx, hostname)
		if err != nil {
			return nil, err
		}
		// Filter to IPv4 only
		var ipv4s []string
		for _, ip := range ips {
			parsed := net.ParseIP(ip)
			if parsed != nil && parsed.To4() != nil {
				ipv4s = append(ipv4s, ip)
			}
		}
		result.Records = ipv4s
		return result, nil

	case "AAAA":
		ips, err := resolver.LookupHost(ctx, hostname)
		if err != nil {
			return nil, err
		}
		// Filter to IPv6 only
		var ipv6s []string
		for _, ip := range ips {
			parsed := net.ParseIP(ip)
			if parsed != nil && parsed.To4() == nil {
				ipv6s = append(ipv6s, ip)
			}
		}
		result.Records = ipv6s
		return result, nil

	case "CNAME":
		cname, err := resolver.LookupCNAME(ctx, hostname)
		if err != nil {
			return nil, err
		}
		result.Records = []string{cname}
		return result, nil

	case "MX":
		mxRecords, err := resolver.LookupMX(ctx, hostname)
		if err != nil {
			return nil, err
		}
		var wiredMXRecords []MXRecordWire
		for _, mx := range mxRecords {
			wiredMXRecords = append(wiredMXRecords, MXRecordWire{Host: mx.Host, Pref: mx.Pref})
		}
		result.MXRecords = wiredMXRecords
		return result, nil

	case "TXT":
		txtRecords, err := resolver.LookupTXT(ctx, hostname)
		if err != nil {
			return nil, err
		}
		result.Records = txtRecords
		return result, nil

	case "NS":
		nsRecords, err := resolver.LookupNS(ctx, hostname)
		if err != nil {
			return nil, err
		}
		var records []string
		for _, ns := range nsRecords {
			records = append(records, ns.Host)
		}
		result.Records = records
		return result, nil

	default:
		return nil, fmt.Errorf("unsupported record type: %s", recordType)
	}
}
