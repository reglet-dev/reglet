package netutil

import (
	"crypto/tls"
)

// TLSConfig returns a secure TLS configuration with TLS 1.2+ minimum.
// This enforces Constitution II: TLS Enforcement requirements.
func TLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			// TLS 1.3 cipher suites (automatically selected when TLS 1.3 is used)
			// TLS 1.2 secure cipher suites
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		},
	}
}

// InsecureTLSConfig returns a TLS configuration that skips certificate verification.
// This should only be used with explicit user consent (--insecure flag).
// WARNING: Using this config disables security protections.
func InsecureTLSConfig() *tls.Config {
	cfg := TLSConfig()
	cfg.InsecureSkipVerify = true
	return cfg
}

// TLSVersionString returns a human-readable TLS version string.
func TLSVersionString(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return "Unknown"
	}
}

// MinTLSVersion returns the minimum required TLS version.
func MinTLSVersion() uint16 {
	return tls.VersionTLS12
}

// MinTLSVersionString returns the minimum required TLS version as a string.
func MinTLSVersionString() string {
	return "TLS 1.2"
}
