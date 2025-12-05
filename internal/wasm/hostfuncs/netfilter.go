package hostfuncs

import (
	"context"
	"fmt"
	"log/slog"
	"net"
)

// IsPrivateOrReservedIP checks if an IP is in private/reserved ranges
// This prevents SSRF attacks by blocking access to:
// - Loopback addresses (127.0.0.0/8, ::1)
// - Private networks (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, fc00::/7)
// - Link-local addresses (169.254.0.0/16, fe80::/10)
// - Multicast addresses (224.0.0.0/4, ff00::/8)
func IsPrivateOrReservedIP(ip net.IP) bool {
	privateRanges := []string{
		"127.0.0.0/8",      // IPv4 loopback
		"10.0.0.0/8",       // RFC1918
		"172.16.0.0/12",    // RFC1918
		"192.168.0.0/16",   // RFC1918
		"169.254.0.0/16",   // Link-local (AWS metadata service!)
		"::1/128",          // IPv6 loopback
		"fc00::/7",         // IPv6 unique local address
		"fe80::/10",        // IPv6 link-local
		"224.0.0.0/4",      // IPv4 multicast
		"ff00::/8",         // IPv6 multicast
	}

	for _, cidr := range privateRanges {
		_, block, err := net.ParseCIDR(cidr)
		if err != nil {
			continue // Skip invalid CIDR
		}
		if block.Contains(ip) {
			return true
		}
	}

	return false
}

// ValidateDestination validates that a hostname is allowed based on capabilities
// - Blocks private/reserved IPs by default (SSRF protection)
// - Allows private IPs if network:outbound:private capability is granted
func ValidateDestination(ctx context.Context, host string, pluginName string, checker *CapabilityChecker) error {
	// Resolve hostname to IP addresses
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("failed to resolve host: %w", err)
	}

	// Check each resolved IP
	for _, ip := range ips {
		if IsPrivateOrReservedIP(ip) {
			// Check if plugin has private network access capability
			if checker != nil {
				if err := checker.Check(pluginName, "network", "outbound:private"); err == nil {
					slog.DebugContext(ctx, "private network access granted via capability",
						"host", host, "ip", ip.String(), "plugin", pluginName)
					return nil // Allowed via capability
				}
			}

			// Blocked - return detailed error
			return fmt.Errorf("destination %s resolves to private/reserved IP %s (requires network:outbound:private capability)", host, ip.String())
		}
	}

	// All IPs are public - allow
	return nil
}
