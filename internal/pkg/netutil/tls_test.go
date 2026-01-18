package netutil_test

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/reglet-dev/reglet/internal/pkg/netutil"
)

func Test_TLSConfig_MinVersion(t *testing.T) {
	cfg := netutil.TLSConfig()

	assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	assert.False(t, cfg.InsecureSkipVerify)
}

func Test_TLSConfig_HasSecureCipherSuites(t *testing.T) {
	cfg := netutil.TLSConfig()

	assert.NotEmpty(t, cfg.CipherSuites)

	// Should include modern cipher suites
	cipherSuites := make(map[uint16]bool)
	for _, suite := range cfg.CipherSuites {
		cipherSuites[suite] = true
	}

	// At least one AES-GCM suite should be present
	hasAESGCM := cipherSuites[tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256] ||
		cipherSuites[tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384] ||
		cipherSuites[tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256] ||
		cipherSuites[tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384]

	assert.True(t, hasAESGCM, "should include at least one AES-GCM cipher suite")
}

func Test_InsecureTLSConfig(t *testing.T) {
	cfg := netutil.InsecureTLSConfig()

	assert.True(t, cfg.InsecureSkipVerify)
	// Should still have TLS 1.2 minimum
	assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
}

func Test_TLSVersionString(t *testing.T) {
	assert.Equal(t, "TLS 1.0", netutil.TLSVersionString(tls.VersionTLS10))
	assert.Equal(t, "TLS 1.1", netutil.TLSVersionString(tls.VersionTLS11))
	assert.Equal(t, "TLS 1.2", netutil.TLSVersionString(tls.VersionTLS12))
	assert.Equal(t, "TLS 1.3", netutil.TLSVersionString(tls.VersionTLS13))
	assert.Equal(t, "Unknown", netutil.TLSVersionString(0))
}

func Test_MinTLSVersion(t *testing.T) {
	assert.Equal(t, uint16(tls.VersionTLS12), netutil.MinTLSVersion())
	assert.Equal(t, "TLS 1.2", netutil.MinTLSVersionString())
}
