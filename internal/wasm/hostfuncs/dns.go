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
	if err := checker.Check("network", "outbound:53"); err != nil {
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
	records, err := performDNSLookup(lookupCtx, request.Hostname, request.Type, request.Nameserver)
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
		Records: records,
	})
}

// performDNSLookup executes the actual DNS lookup based on record type
func performDNSLookup(ctx context.Context, hostname string, recordType string, nameserver string) ([]string, error) {
	var resolver *net.Resolver

	if nameserver != "" {
		// Use custom resolver
		resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
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
		return ipv4s, nil

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
		return ipv6s, nil

	case "CNAME":
		cname, err := resolver.LookupCNAME(ctx, hostname)
		if err != nil {
			return nil, err
		}
		return []string{cname}, nil

	case "MX":
		mxRecords, err := resolver.LookupMX(ctx, hostname)
		if err != nil {
			return nil, err
		}
		var records []string
		for _, mx := range mxRecords {
			records = append(records, fmt.Sprintf("%d %s", mx.Pref, mx.Host))
		}
		return records, nil

	case "TXT":
		txtRecords, err := resolver.LookupTXT(ctx, hostname)
		if err != nil {
			return nil, err
		}
		return txtRecords, nil

	case "NS":
		nsRecords, err := resolver.LookupNS(ctx, hostname)
		if err != nil {
			return nil, err
		}
		var records []string
		for _, ns := range nsRecords {
			records = append(records, ns.Host)
		}
		return records, nil

	default:
		return nil, fmt.Errorf("unsupported record type: %s", recordType)
	}
}
