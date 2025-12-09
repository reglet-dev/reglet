package hostfuncs

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsPrivateOrReservedIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		ip        string
		isPrivate bool
	}{
		// Loopback addresses
		{"IPv4 loopback", "127.0.0.1", true},
		{"IPv4 loopback network", "127.255.255.255", true},
		{"IPv6 loopback", "::1", true},

		// Private networks (RFC1918)
		{"10.0.0.0/8 start", "10.0.0.0", true},
		{"10.0.0.0/8 end", "10.255.255.255", true},
		{"10.0.0.0/8 middle", "10.123.45.67", true},
		{"172.16.0.0/12 start", "172.16.0.0", true},
		{"172.16.0.0/12 end", "172.31.255.255", true},
		{"172.16.0.0/12 middle", "172.20.1.1", true},
		{"192.168.0.0/16 start", "192.168.0.0", true},
		{"192.168.0.0/16 end", "192.168.255.255", true},
		{"192.168.0.0/16 middle", "192.168.1.1", true},

		// Link-local (AWS metadata service)
		{"169.254.0.0/16 start", "169.254.0.0", true},
		{"169.254.169.254 (AWS)", "169.254.169.254", true},
		{"169.254.0.0/16 end", "169.254.255.255", true},

		// IPv6 private
		{"IPv6 unique local", "fc00::1", true},
		{"IPv6 link-local", "fe80::1", true},

		// Multicast
		{"IPv4 multicast start", "224.0.0.0", true},
		{"IPv4 multicast end", "239.255.255.255", true},
		{"IPv6 multicast", "ff02::1", true},

		// Public IPs (should NOT be private)
		{"Public IP Google DNS", "8.8.8.8", false},
		{"Public IP Cloudflare", "1.1.1.1", false},
		{"Public IP example.com range", "93.184.216.34", false},
		{"Public IPv6 Google", "2001:4860:4860::8888", false},

		// Edge cases near private ranges
		{"Just before 10.0.0.0", "9.255.255.255", false},
		{"Just after 10.255.255.255", "11.0.0.0", false},
		{"Just before 172.16.0.0", "172.15.255.255", false},
		{"Just after 172.31.255.255", "172.32.0.0", false},
		{"Just before 192.168.0.0", "192.167.255.255", false},
		{"Just after 192.168.255.255", "192.169.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ip := net.ParseIP(tt.ip)
			require.NotNil(t, ip, "failed to parse IP: %s", tt.ip)

			result := IsPrivateOrReservedIP(ip)
			assert.Equal(t, tt.isPrivate, result, "IP %s private status mismatch", tt.ip)
		})
	}
}

func TestValidateDestination_PublicIPs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	checker := NewCapabilityChecker(map[string][]Capability{
		"test-plugin": {
			{Kind: "network", Pattern: "outbound:80"},
		},
	})

	tests := []struct {
		name     string
		host     string
		shouldOK bool
	}{
		{"public IPv4 address", "8.8.8.8", true},
		{"public IPv6 address", "2001:4860:4860::8888", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateDestination(ctx, tt.host, "test-plugin", checker)
			if tt.shouldOK {
				assert.NoError(t, err, "public IP should be allowed")
			} else {
				assert.Error(t, err, "expected error for this destination")
			}
		})
	}
}

func TestValidateDestination_PrivateIPs_Blocked(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	// Checker WITHOUT network:outbound:private capability
	checker := NewCapabilityChecker(map[string][]Capability{
		"test-plugin": {
			{Kind: "network", Pattern: "outbound:80"},
		},
	})

	tests := []struct {
		name string
		host string
	}{
		{"localhost IPv4", "127.0.0.1"},
		{"localhost IPv6", "::1"},
		{"private 10.x", "10.0.0.1"},
		{"private 192.168.x", "192.168.1.1"},
		{"private 172.16.x", "172.16.0.1"},
		{"AWS metadata", "169.254.169.254"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateDestination(ctx, tt.host, "test-plugin", checker)
			require.Error(t, err, "private IP should be blocked without capability")
			assert.Contains(t, err.Error(), "private/reserved IP")
			assert.Contains(t, err.Error(), "network:outbound:private")
		})
	}
}

func TestValidateDestination_PrivateIPs_AllowedWithCapability(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	// Checker WITH network:outbound:private capability
	checker := NewCapabilityChecker(map[string][]Capability{
		"test-plugin": {
			{Kind: "network", Pattern: "outbound:80"},
			{Kind: "network", Pattern: "outbound:private"},
		},
	})

	tests := []struct {
		name string
		host string
	}{
		{"localhost IPv4", "127.0.0.1"},
		{"localhost IPv6", "::1"},
		{"private 10.x", "10.0.0.1"},
		{"private 192.168.x", "192.168.1.1"},
		{"private 172.16.x", "172.16.0.1"},
		{"AWS metadata", "169.254.169.254"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateDestination(ctx, tt.host, "test-plugin", checker)
			assert.NoError(t, err, "private IP should be allowed with network:outbound:private capability")
		})
	}
}

func TestValidateDestination_DNSResolutionError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	checker := NewCapabilityChecker(map[string][]Capability{
		"test-plugin": {
			{Kind: "network", Pattern: "outbound:80"},
		},
	})

	// Use a definitely non-existent domain
	err := ValidateDestination(ctx, "this-domain-absolutely-does-not-exist.invalid", "test-plugin", checker)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve host")
}

func TestValidateDestination_NilChecker(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// With nil checker, private IPs should still be blocked
	err := ValidateDestination(ctx, "127.0.0.1", "test-plugin", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private/reserved IP")

	// Public IPs should be allowed
	err = ValidateDestination(ctx, "8.8.8.8", "test-plugin", nil)
	assert.NoError(t, err)
}
