// Package netutil provides HTTP security utilities for safe network operations.
// This package is designed to be reusable across the codebase without
// dependencies on domain, application, or infrastructure layers.
package netutil

import (
	"net"
)

// privateIPBlocks contains all private/reserved IP ranges that should be
// blocked by default to prevent SSRF attacks.
var privateIPBlocks []*net.IPNet

func init() {
	// Initialize private IP blocks
	// These are the ranges that should be blocked by default:
	// - 10.0.0.0/8 (Class A private)
	// - 172.16.0.0/12 (Class B private)
	// - 192.168.0.0/16 (Class C private)
	// - 169.254.0.0/16 (link-local)
	// - 127.0.0.0/8 (loopback)
	// - ::1/128 (IPv6 loopback)
	// - fc00::/7 (IPv6 unique local)
	// - fe80::/10 (IPv6 link-local)
	cidrs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
		"127.0.0.0/8",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}

	for _, cidr := range cidrs {
		_, block, err := net.ParseCIDR(cidr)
		if err != nil {
			// This should never happen with hardcoded CIDRs
			panic("invalid CIDR in privateIPBlocks: " + cidr)
		}
		privateIPBlocks = append(privateIPBlocks, block)
	}
}

// IsPrivateIP returns true if the given IP address is in a private/reserved range.
// This includes:
//   - 10.0.0.0/8 (Class A private)
//   - 172.16.0.0/12 (Class B private)
//   - 192.168.0.0/16 (Class C private)
//   - 169.254.0.0/16 (link-local)
//   - 127.0.0.0/8 (loopback)
//   - ::1/128 (IPv6 loopback)
//   - fc00::/7 (IPv6 unique local)
//   - fe80::/10 (IPv6 link-local)
func IsPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}

	// Check standard Go library methods first for common cases
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// Check against our private blocks
	for _, block := range privateIPBlocks {
		if block.Contains(ip) {
			return true
		}
	}

	return false
}

// IsPrivateIPString parses an IP string and checks if it's private.
// Returns false if the string is not a valid IP address.
func IsPrivateIPString(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	return IsPrivateIP(ip)
}

// IsPrivateHost resolves a hostname and checks if any of its IPs are private.
// Returns true if the host resolves to any private IP.
// Returns false if the host cannot be resolved or has no private IPs.
func IsPrivateHost(host string) (bool, net.IP) {
	// First check if it's already an IP address
	if ip := net.ParseIP(host); ip != nil {
		if IsPrivateIP(ip) {
			return true, ip
		}
		return false, nil
	}

	// Resolve hostname
	ips, err := net.LookupIP(host)
	if err != nil {
		return false, nil
	}

	for _, ip := range ips {
		if IsPrivateIP(ip) {
			return true, ip
		}
	}

	return false, nil
}

// PrivateIPRanges returns a human-readable list of blocked IP ranges.
// Useful for error messages and documentation.
func PrivateIPRanges() []string {
	return []string{
		"10.0.0.0/8 (Class A private)",
		"172.16.0.0/12 (Class B private)",
		"192.168.0.0/16 (Class C private)",
		"169.254.0.0/16 (link-local)",
		"127.0.0.0/8 (loopback)",
		"::1/128 (IPv6 loopback)",
		"fc00::/7 (IPv6 unique local)",
		"fe80::/10 (IPv6 link-local)",
	}
}
