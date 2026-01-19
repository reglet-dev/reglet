package netutil_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/reglet-dev/reglet/internal/pkg/netutil"
)

func Test_SecureDialer_BlocksPrivateIP(t *testing.T) {
	dialer := &netutil.SecureDialer{
		AllowPrivateNetwork: false,
	}

	// Try to connect to localhost (private)
	_, err := dialer.DialContext(context.Background(), "tcp", "127.0.0.1:80")
	require.Error(t, err)
	assert.True(t, netutil.IsPrivateIPError(err))
	assert.Contains(t, err.Error(), "private IP")
	assert.Contains(t, err.Error(), "SSRF protection")
}

func Test_SecureDialer_AllowsPrivateIPWithFlag(t *testing.T) {
	dialer := &netutil.SecureDialer{
		AllowPrivateNetwork: true,
	}

	// This will fail to connect (no server), but shouldn't error on SSRF
	_, err := dialer.DialContext(context.Background(), "tcp", "127.0.0.1:12345")

	// Should NOT be a PrivateIPError - it should be a connection refused error
	assert.False(t, netutil.IsPrivateIPError(err))
}

func Test_SecureDialer_CallsOnPrivateIPBlocked(t *testing.T) {
	var blockedIP net.IP
	dialer := &netutil.SecureDialer{
		AllowPrivateNetwork: false,
		OnPrivateIPBlocked: func(ip net.IP) {
			blockedIP = ip
		},
	}

	_, err := dialer.DialContext(context.Background(), "tcp", "10.0.0.1:80")
	require.Error(t, err)
	assert.NotNil(t, blockedIP)
	assert.Equal(t, "10.0.0.1", blockedIP.String())
}

func Test_SecureDialer_InvalidAddress(t *testing.T) {
	dialer := &netutil.SecureDialer{}

	_, err := dialer.DialContext(context.Background(), "tcp", "invalid-no-port")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid address")
}

func Test_PrivateIPError(t *testing.T) {
	err := &netutil.PrivateIPError{IP: net.ParseIP("10.0.0.1")}

	assert.Contains(t, err.Error(), "10.0.0.1")
	assert.Contains(t, err.Error(), "--allow-private-network")
	assert.True(t, netutil.IsPrivateIPError(err))
	assert.False(t, netutil.IsPrivateIPError(nil))
	assert.False(t, netutil.IsPrivateIPError(assert.AnError))
}
