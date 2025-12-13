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
		"127.0.0.0/8",    // IPv4 loopback
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"169.254.0.0/16", // Link-local (AWS metadata service!)
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 unique local address
		"fe80::/10",      // IPv6 link-local
		"224.0.0.0/4",    // IPv4 multicast
		"ff00::/8",       // IPv6 multicast
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

// resolveAndValidate resolves a hostname to an IP address and validates it
// Returns a validated IP address string to prevent DNS rebinding attacks
// This function resolves DNS ONCE, validates the IP, then returns it for direct connection
func resolveAndValidate(ctx context.Context, host string, pluginName string, checker *CapabilityChecker) (string, error) {
	// Check if host is already an IP address
	if ip := net.ParseIP(host); ip != nil {
		// Already an IP - validate it directly
		if IsPrivateOrReservedIP(ip) {
			if checker != nil {
				if err := checker.Check(pluginName, "network", "outbound:private"); err == nil {
					return host, nil // Allowed via capability
				}
			}
			return "", fmt.Errorf("destination IP %s is private/reserved (requires network:outbound:private capability)", ip.String())
		}
		return host, nil
	}

	// Resolve hostname to IP addresses
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return "", fmt.Errorf("failed to resolve host: %w", err)
	}

	if len(ips) == 0 {
		return "", fmt.Errorf("no IP addresses found for host %s", host)
	}

	// Use first resolved IP and validate it
	ip := ips[0]
	if IsPrivateOrReservedIP(ip) {
		if checker != nil {
			if err := checker.Check(pluginName, "network", "outbound:private"); err == nil {
				slog.DebugContext(ctx, "private network access granted via capability",
					"host", host, "ip", ip.String(), "plugin", pluginName)
				return ip.String(), nil // Allowed via capability
			}
		}
		return "", fmt.Errorf("destination %s resolves to private/reserved IP %s (requires network:outbound:private capability)", host, ip.String())
	}

	// Return validated IP as string
	return ip.String(), nil
}
