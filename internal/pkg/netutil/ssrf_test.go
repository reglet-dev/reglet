package netutil_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/reglet-dev/reglet/internal/pkg/netutil"
)

func Test_IsPrivateIP(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		// IPv4 private ranges
		{"Class A private - start", "10.0.0.0", true},
		{"Class A private - middle", "10.128.0.1", true},
		{"Class A private - end", "10.255.255.255", true},
		{"Class B private - start", "172.16.0.0", true},
		{"Class B private - middle", "172.20.0.1", true},
		{"Class B private - end", "172.31.255.255", true},
		{"Class C private - start", "192.168.0.0", true},
		{"Class C private - middle", "192.168.1.1", true},
		{"Class C private - end", "192.168.255.255", true},
		{"Link-local", "169.254.1.1", true},
		{"Loopback", "127.0.0.1", true},
		{"Loopback - alternative", "127.0.0.2", true},

		// IPv6 private ranges
		{"IPv6 loopback", "::1", true},
		{"IPv6 unique local", "fc00::1", true},
		{"IPv6 unique local - fd", "fd00::1", true},
		{"IPv6 link-local", "fe80::1", true},

		// Public IPs (should return false)
		{"Public IPv4 - Google DNS", "8.8.8.8", false},
		{"Public IPv4 - Cloudflare", "1.1.1.1", false},
		{"Public IPv4 - random", "93.184.216.34", false},
		{"Public IPv6", "2001:4860:4860::8888", false},

		// Edge cases - just outside private ranges
		{"Just before Class B private", "172.15.255.255", false},
		{"Just after Class B private", "172.32.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			assert.NotNil(t, ip, "failed to parse IP: %s", tt.ip)
			assert.Equal(t, tt.want, netutil.IsPrivateIP(ip))
		})
	}
}

func Test_IsPrivateIP_NilIP(t *testing.T) {
	assert.False(t, netutil.IsPrivateIP(nil))
}

func Test_IsPrivateIPString(t *testing.T) {
	assert.True(t, netutil.IsPrivateIPString("10.0.0.1"))
	assert.True(t, netutil.IsPrivateIPString("127.0.0.1"))
	assert.False(t, netutil.IsPrivateIPString("8.8.8.8"))
	assert.False(t, netutil.IsPrivateIPString("invalid"))
	assert.False(t, netutil.IsPrivateIPString(""))
}

func Test_PrivateIPRanges(t *testing.T) {
	ranges := netutil.PrivateIPRanges()
	assert.Len(t, ranges, 8)
	assert.Contains(t, ranges[0], "10.0.0.0/8")
}
