package hostfuncs

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/tetratelabs/wazero/api"
)

// DNSLookup performs DNS resolution on behalf of the plugin
// Parameters: hostnamePtr (i32), hostnameLen (i32), recordTypePtr (i32), recordTypeLen (i32)
// Returns: resultPtr (i32) - pointer to JSON result in WASM memory
func DNSLookup(ctx context.Context, mod api.Module, stack []uint64, checker *CapabilityChecker) {
	hostnamePtr := uint32(stack[0])
	hostnameLen := uint32(stack[1])
	recordTypePtr := uint32(stack[2])
	recordTypeLen := uint32(stack[3])

	// 1. Check capability
	if err := checker.Check("network", "outbound:53"); err != nil {
		stack[0] = uint64(writeError(ctx, mod, "permission denied: "+err.Error()))
		return
	}

	// 2. Read hostname from WASM memory
	hostnameBytes, ok := mod.Memory().Read(hostnamePtr, hostnameLen)
	if !ok {
		stack[0] = uint64(writeError(ctx, mod, "failed to read hostname from memory"))
		return
	}
	hostname := string(hostnameBytes)

	// 3. Read record type from WASM memory
	recordTypeBytes, ok := mod.Memory().Read(recordTypePtr, recordTypeLen)
	if !ok {
		stack[0] = uint64(writeError(ctx, mod, "failed to read record type from memory"))
		return
	}
	recordType := string(recordTypeBytes)

	// 4. Validate input
	if hostname == "" {
		stack[0] = uint64(writeError(ctx, mod, "hostname cannot be empty"))
		return
	}

	// 5. Perform DNS lookup with timeout
	lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	records, err := performDNSLookup(lookupCtx, hostname, recordType)
	if err != nil {
		stack[0] = uint64(writeError(ctx, mod, fmt.Sprintf("DNS lookup failed: %v", err)))
		return
	}

	// 6. Write success response to WASM memory and return pointer
	stack[0] = uint64(writeSuccess(ctx, mod, records))
}

// performDNSLookup executes the actual DNS lookup based on record type
func performDNSLookup(ctx context.Context, hostname string, recordType string) ([]string, error) {
	resolver := net.DefaultResolver

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
