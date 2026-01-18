package netutil

import (
	"context"
	"fmt"
	"net"
	"time"
)

// SecureDialer provides DNS pinning and SSRF protection for network connections.
// It resolves DNS once and pins the IP for the duration of the connection,
// preventing DNS rebinding attacks.
type SecureDialer struct {
	// AllowPrivateNetwork permits connections to private IP ranges.
	// Default: false (blocked with callback notification).
	AllowPrivateNetwork bool

	// OnPrivateIPBlocked is called when a connection to a private IP is blocked.
	// The callback receives the blocked IP address.
	OnPrivateIPBlocked func(ip net.IP)

	// OnDNSPinning is called when DNS is resolved and pinned.
	// The callback receives the hostname and pinned IP.
	OnDNSPinning func(host string, ip net.IP)

	// Timeout for the dial operation.
	// Default: 30s if zero.
	Timeout time.Duration

	// Resolver is the DNS resolver to use.
	// Default: net.DefaultResolver if nil.
	Resolver *net.Resolver
}

// DialContext connects to the address with DNS pinning and SSRF protection.
// It resolves DNS once, validates against private IP ranges, and connects
// using the pinned IP to prevent DNS rebinding attacks.
func (d *SecureDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid address %q: %w", addr, err)
	}

	// Check if it's already an IP address
	if ip := net.ParseIP(host); ip != nil {
		if err := d.validateIP(ip); err != nil {
			return nil, err
		}
		return d.dialIP(ctx, network, ip, port)
	}

	// Resolve DNS
	resolver := d.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}

	ips, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("DNS lookup failed for %q: %w", host, err)
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("no IP addresses found for %q", host)
	}

	// Use the first valid IP (prefer IPv4 for compatibility)
	var selectedIP net.IP
	for _, ipAddr := range ips {
		ip := ipAddr.IP
		if ip.To4() != nil {
			selectedIP = ip
			break
		}
	}
	if selectedIP == nil {
		selectedIP = ips[0].IP
	}

	// Notify about DNS pinning
	if d.OnDNSPinning != nil {
		d.OnDNSPinning(host, selectedIP)
	}

	// Validate the resolved IP
	if err := d.validateIP(selectedIP); err != nil {
		return nil, err
	}

	return d.dialIP(ctx, network, selectedIP, port)
}

// validateIP checks if the IP is allowed based on SSRF protection settings.
func (d *SecureDialer) validateIP(ip net.IP) error {
	if IsPrivateIP(ip) && !d.AllowPrivateNetwork {
		if d.OnPrivateIPBlocked != nil {
			d.OnPrivateIPBlocked(ip)
		}
		return &PrivateIPError{IP: ip}
	}
	return nil
}

// dialIP connects to the specified IP and port.
func (d *SecureDialer) dialIP(ctx context.Context, network string, ip net.IP, port string) (net.Conn, error) {
	timeout := d.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	dialer := &net.Dialer{
		Timeout: timeout,
	}

	addr := net.JoinHostPort(ip.String(), port)
	return dialer.DialContext(ctx, network, addr)
}

// PrivateIPError is returned when a connection to a private IP is blocked.
type PrivateIPError struct {
	IP net.IP
}

func (e *PrivateIPError) Error() string {
	return fmt.Sprintf("connection to private IP %s blocked (SSRF protection); use --allow-private-network to permit", e.IP)
}

// IsPrivateIPError returns true if the error is a PrivateIPError.
func IsPrivateIPError(err error) bool {
	_, ok := err.(*PrivateIPError)
	return ok
}
