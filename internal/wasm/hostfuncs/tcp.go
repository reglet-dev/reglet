package hostfuncs

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/tetratelabs/wazero/api"
)

// TCPConnect performs TCP connection tests on behalf of the plugin
//
// Parameters (via WASM stack):
//   - hostPtr, hostLen: Target host
//   - portPtr, portLen: Target port
//   - timeoutMs: Connection timeout in milliseconds
//   - useTLS: 1 for TLS, 0 for plain TCP
//
// Returns: Pointer to JSON result in WASM memory:
//
//	{
//	  "status": true,
//	  "connected": true,
//	  "response_time_ms": 45,
//	  "address": "example.com:443",
//	  "tls_version": "TLS 1.3",
//	  "tls_cipher_suite": "TLS_AES_128_GCM_SHA256"
//	}
//
// or error:
//
//	{"status": false, "error": "connection refused"}
func TCPConnect(ctx context.Context, mod api.Module, stack []uint64, checker *CapabilityChecker) {
	hostPtr := uint32(stack[0])
	hostLen := uint32(stack[1])
	portPtr := uint32(stack[2])
	portLen := uint32(stack[3])
	timeoutMs := uint32(stack[4])
	useTLS := uint32(stack[5])

	// Read host from WASM memory
	hostBytes, ok := mod.Memory().Read(hostPtr, hostLen)
	if !ok {
		stack[0] = uint64(writeError(ctx, mod, "failed to read host from memory"))
		return
	}
	host := string(hostBytes)

	// Read port from WASM memory
	portBytes, ok := mod.Memory().Read(portPtr, portLen)
	if !ok {
		stack[0] = uint64(writeError(ctx, mod, "failed to read port from memory"))
		return
	}
	port := string(portBytes)

	// Check capability for outbound TCP
	if err := checker.Check("network", fmt.Sprintf("outbound:%s", port)); err != nil {
		stack[0] = uint64(writeError(ctx, mod, fmt.Sprintf("permission denied: %v", err)))
		return
	}

	// Validate timeout (1ms - 60s)
	if timeoutMs == 0 {
		timeoutMs = 5000 // Default 5 seconds
	}
	if timeoutMs > 60000 {
		timeoutMs = 60000 // Max 60 seconds
	}

	// Perform TCP connection test
	timeout := time.Duration(timeoutMs) * time.Millisecond
	connectCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	result, err := performTCPConnect(connectCtx, host, port, useTLS == 1)
	responseTime := time.Since(start).Milliseconds()

	if err != nil {
		stack[0] = uint64(writeError(ctx, mod, fmt.Sprintf("TCP connection failed: %v", err)))
		return
	}

	// Add response time to result
	result["response_time_ms"] = responseTime

	// Return success
	stack[0] = uint64(writeJSON(ctx, mod, result))
}

// performTCPConnect executes the actual TCP connection test
func performTCPConnect(ctx context.Context, host, port string, useTLS bool) (map[string]interface{}, error) {
	address := net.JoinHostPort(host, port)

	result := map[string]interface{}{
		"status":    true,
		"connected": false,
		"address":   address,
	}

	// Create dialer with context
	dialer := &net.Dialer{}

	if !useTLS {
		// Plain TCP connection
		conn, err := dialer.DialContext(ctx, "tcp", address)
		if err != nil {
			return nil, fmt.Errorf("connection failed: %w", err)
		}
		defer conn.Close()

		result["connected"] = true
		result["remote_addr"] = conn.RemoteAddr().String()
		result["local_addr"] = conn.LocalAddr().String()

		return result, nil
	}

	// TLS connection
	tlsConfig := &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", address, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("TLS connection failed: %w", err)
	}
	defer conn.Close()

	// Get TLS connection state
	state := conn.ConnectionState()

	result["connected"] = true
	result["remote_addr"] = conn.RemoteAddr().String()
	result["local_addr"] = conn.LocalAddr().String()
	result["tls"] = true
	result["tls_version"] = tlsVersionString(state.Version)
	result["tls_cipher_suite"] = tls.CipherSuiteName(state.CipherSuite)
	result["tls_server_name"] = state.ServerName
	result["tls_handshake_complete"] = state.HandshakeComplete

	// Certificate info (basic)
	if len(state.PeerCertificates) > 0 {
		cert := state.PeerCertificates[0]
		result["tls_cert_subject"] = cert.Subject.String()
		result["tls_cert_issuer"] = cert.Issuer.String()
		result["tls_cert_not_before"] = cert.NotBefore.Format(time.RFC3339)
		result["tls_cert_not_after"] = cert.NotAfter.Format(time.RFC3339)
		result["tls_cert_dns_names"] = cert.DNSNames
	}

	return result, nil
}

// tlsVersionString converts TLS version constant to string
func tlsVersionString(version uint16) string {
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
		return fmt.Sprintf("Unknown (0x%04X)", version)
	}
}
