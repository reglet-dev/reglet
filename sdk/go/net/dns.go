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
)

// Define the host function signature for DNS lookups.
// This matches the signature defined in internal/wasm/hostfuncs/registry.go.
//go:wasmimport reglet_host dns_lookup
func host_dns_lookup(requestPacked uint64) uint64

// WasmResolver implements net.Resolver functionality for the WASM environment.
type WasmResolver struct{
	// Nameserver is the address of the nameserver to use for resolution (e.g. "8.8.8.8:53").
	// If empty, the host's default resolver is used.
	Nameserver string
}

// LookupHost resolves IP addresses for a given host using the host function.
func (r *WasmResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	// Try A records
	addrsA, errA := r.lookup(ctx, host, "A")
	if errA != nil {
		return nil, errA
	}

	// Try AAAA records
	// We don't fail if AAAA fails, unless A also failed (but A returned nil error above).
	// Actually, standard LookupHost behavior is to return what it finds.
	// If A lookup succeeded (even with 0 records), we try AAAA.
	addrsAAAA, errAAAA := r.lookup(ctx, host, "AAAA")
	if errAAAA != nil {
		// Use slog to log the error but don't fail the whole lookup if A succeeded?
		// Standard behavior: if one fails, it might be a network issue.
		// But typically if A succeeds, we return those.
		// Let's be safe: return error if AAAA fails?
		// If the host doesn't have AAAA, lookup should return empty list, not error.
		// So real error means DNS failure.
		return nil, errAAAA
	}

	return append(addrsA, addrsAAAA...), nil
}

// LookupIPAddr resolves IP addresses for a given host using the host function.
func (r *WasmResolver) LookupIPAddr(ctx context.Context, host string) ([]stdnet.IPAddr, error) {
	records, err := r.lookup(ctx, host, "A") // Get A records
	if err != nil {
		return nil, err
	}
	recordsIPv6, err := r.lookup(ctx, host, "AAAA") // Get AAAA records
	if err != nil {
		return nil, err
	}
	records = append(records, recordsIPv6...)

	var ipAddrs []stdnet.IPAddr
	for _, rec := range records {
		if ip := stdnet.ParseIP(rec); ip != nil {
			ipAddrs = append(ipAddrs, stdnet.IPAddr{IP: ip})
		}
	}
	return ipAddrs, nil
}

// lookup performs the actual DNS query via the host function.
func (r *WasmResolver) lookup(ctx context.Context, hostname, recordType string) ([]string, error) {
	wireCtx := createContextWireFormat(ctx)
	request := DNSRequestWire{
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

	var response DNSResponseWire
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		return nil, fmt.Errorf("sdk: failed to unmarshal DNS response: %w", err)
	}

	if response.Error != nil {
		return nil, response.Error // Convert structured error to Go error
	}

	return response.Records, nil
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
	records, err := r.lookup(ctx, host, "CNAME")
	if err != nil {
		return "", err
	}
	if len(records) == 0 {
		return "", fmt.Errorf("no CNAME record found")
	}
	return records[0], nil
}

// LookupMX returns MX records as strings "Pref Host"
func (r *WasmResolver) LookupMX(ctx context.Context, host string) ([]string, error) {
	return r.lookup(ctx, host, "MX")
}

// LookupTXT returns TXT records
func (r *WasmResolver) LookupTXT(ctx context.Context, host string) ([]string, error) {
	return r.lookup(ctx, host, "TXT")
}

// LookupNS returns NS records (nameservers)
func (r *WasmResolver) LookupNS(ctx context.Context, host string) ([]string, error) {
	return r.lookup(ctx, host, "NS")
}

// Exported helper for plugins to use instead of net.LookupHost
func LookupHost(ctx context.Context, host string) ([]string, error) {
	r := &WasmResolver{}
	return r.LookupHost(ctx, host)
}

// LookupMX returns MX records as strings "Pref Host"
func LookupMX(ctx context.Context, host string) ([]string, error) {
	r := &WasmResolver{}
	return r.LookupMX(ctx, host)
}

// LookupCNAME returns the canonical name for the given host
func LookupCNAME(ctx context.Context, host string) (string, error) {
	r := &WasmResolver{}
	return r.LookupCNAME(ctx, host)
}

// LookupTXT returns TXT records
func LookupTXT(ctx context.Context, host string) ([]string, error) {
	r := &WasmResolver{}
	return r.LookupTXT(ctx, host)
}

// LookupNS returns NS records (nameservers)
func LookupNS(ctx context.Context, host string) ([]string, error) {
	r := &WasmResolver{}
	return r.LookupNS(ctx, host)
}