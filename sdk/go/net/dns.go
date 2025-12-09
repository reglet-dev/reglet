//go:build wasip1

package net

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	stdnet "net"
	"time"

	"github.com/whiskeyjimbo/reglet/sdk/internal/abi"
	_ "github.com/whiskeyjimbo/reglet/sdk/log" // Initialize WASM logging handler
	"github.com/whiskeyjimbo/reglet/wireformat"
)

// Define the host function signature for DNS lookups.
// This matches the signature defined in internal/wasm/hostfuncs/registry.go.
//
//go:wasmimport reglet_host dns_lookup
func host_dns_lookup(requestPacked uint64) uint64

// WasmResolver implements net.Resolver functionality for the WASM environment.
type WasmResolver struct {
	// Nameserver is the address of the nameserver to use for resolution (e.g. "8.8.8.8:53").
	// If empty, the host's default resolver is used.
	Nameserver string
}

// LookupHost resolves IP addresses for a given host using the host function.
func (r *WasmResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	resp, err := r.lookup(ctx, host, "A")
	if err != nil {
		return nil, err
	}
	recordsA := resp.Records

	resp, err = r.lookup(ctx, host, "AAAA")
	if err != nil {
		return nil, err
	}
	recordsAAAA := resp.Records

	return append(recordsA, recordsAAAA...), nil
}

// LookupIPAddr resolves IP addresses for a given host using the host function.
func (r *WasmResolver) LookupIPAddr(ctx context.Context, host string) ([]stdnet.IPAddr, error) {
	resp, err := r.lookup(ctx, host, "A") // Get A records
	if err != nil {
		return nil, err
	}
	records := resp.Records

	resp, err = r.lookup(ctx, host, "AAAA") // Get AAAA records
	if err != nil {
		return nil, err
	}
	records = append(records, resp.Records...)

	var ipAddrs []stdnet.IPAddr
	for _, rec := range records {
		if ip := stdnet.ParseIP(rec); ip != nil {
			ipAddrs = append(ipAddrs, stdnet.IPAddr{IP: ip})
		}
	}
	return ipAddrs, nil
}

// lookup performs the actual DNS query via the host function.
func (r *WasmResolver) lookup(ctx context.Context, hostname, recordType string) (*wireformat.DNSResponseWire, error) {
	wireCtx := createContextWireFormat(ctx)
	request := wireformat.DNSRequestWire{ // Use wireformat's DNSRequestWire
		Context:    wireCtx,
		Hostname:   hostname,
		Type:       recordType,
		Nameserver: r.Nameserver,
	}

	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("sdk: failed to marshal DNS request: %w", err)
	}

	// Call the host function
	responsePacked := host_dns_lookup(abi.PtrFromBytes(requestBytes))

	// Read and unmarshal the response
	responseBytes := abi.BytesFromPtr(responsePacked)
	abi.DeallocatePacked(responsePacked) // Free memory on Guest side (allocated by Host for result)

	var response wireformat.DNSResponseWire
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		return nil, fmt.Errorf("sdk: failed to unmarshal DNS response: %w", err)
	}

	if response.Error != nil {
		return nil, response.Error // Convert structured error to Go error
	}

	return &response, nil
}

// init configures the default resolver to use our WasmResolver.
func init() {
	// Set the default resolver for standard library net calls.
	// This ensures that net.LookupHost, net.LookupIP, and other functions that use the default resolver,
	// will use our WASM-aware implementation.
	stdnet.DefaultResolver = &stdnet.Resolver{
		PreferGo: true, // Use Go's native resolver implementation
		// We implement LookupIPAddr directly to handle A/AAAA lookups through hostfuncs.
		// For other lookup types (MX, TXT, etc.), plugin authors will need to call specific
		// SDK functions (e.g., sdknet.LookupMX) if we don't implement them here directly.

		// NOTE: 'LookupIPAddr' is a method, not a field we can set on the struct literal.
		// net.Resolver struct only has PreferGo (bool) and Dial (func).
		// To customize LookupIPAddr behavior, we rely on PreferGo=true and the Dial function intercepting network traffic.
		// BUT, since we cannot easily intercept the DNS protocol parsing inside net.Resolver via Dial without a full DNS server stub,
		// we are removing the attempt to patch LookupIPAddr here.

		// Plugins MUST use the sdk/net package directly for lookups if they want WASM host function support.
		// Standard net.LookupHost will likely fail or try to dial on prohibited ports.

		Dial: func(ctx context.Context, network, address string) (stdnet.Conn, error) {
			slog.WarnContext(ctx, "sdk: net.DefaultResolver.Dial called, not implemented via hostfunc", "network", network, "address", address)
			return (&stdnet.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, address)
		},
	}
	slog.Info("Reglet SDK: DNS resolver initialized (partial shim).")
}

// LookupCNAME returns the canonical name for the given host
func (r *WasmResolver) LookupCNAME(ctx context.Context, host string) (string, error) {
	resp, err := r.lookup(ctx, host, "CNAME")
	if err != nil {
		return "", err
	}
	if len(resp.Records) == 0 {
		return "", fmt.Errorf("no CNAME record found")
	}
	return resp.Records[0], nil
}

// LookupMX returns MX records as strings "Pref Host" (for compatibility)
func (r *WasmResolver) LookupMX(ctx context.Context, host string) ([]string, error) {
	resp, err := r.lookup(ctx, host, "MX")
	if err != nil {
		return nil, err
	}
	var records []string
	for _, mx := range resp.MXRecords {
		records = append(records, fmt.Sprintf("%d %s", mx.Pref, mx.Host))
	}
	return records, nil
}

// LookupMXRecords returns structured MX records
func (r *WasmResolver) LookupMXRecords(ctx context.Context, host string) ([]wireformat.MXRecordWire, error) {
	resp, err := r.lookup(ctx, host, "MX")
	if err != nil {
		return nil, err
	}
	return resp.MXRecords, nil
}

// LookupTXT returns TXT records
func (r *WasmResolver) LookupTXT(ctx context.Context, host string) ([]string, error) {
	resp, err := r.lookup(ctx, host, "TXT")
	if err != nil {
		return nil, err
	}
	return resp.Records, nil
}

// LookupNS returns NS records (nameservers)
func (r *WasmResolver) LookupNS(ctx context.Context, host string) ([]string, error) {
	resp, err := r.lookup(ctx, host, "NS")
	if err != nil {
		return nil, err
	}
	return resp.Records, nil
}
