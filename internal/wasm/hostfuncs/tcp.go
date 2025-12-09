package hostfuncs

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/tetratelabs/wazero/api"
)

// TCPConnect performs TCP connection tests on behalf of the plugin.
// It receives a packed uint64 (ptr+len) pointing to a JSON-encoded TCPRequestWire.
// It returns a packed uint64 (ptr+len) pointing to a JSON-encoded TCPResponseWire.
func TCPConnect(ctx context.Context, mod api.Module, stack []uint64, checker *CapabilityChecker) {
	// Stack contains a single uint64 which is packed ptr+len of the request.
	requestPacked := stack[0]
	ptr, length := unpackPtrLen(requestPacked)

	requestBytes, ok := mod.Memory().Read(ptr, length)
	if !ok {
		// Critical error, Host could not read Guest memory.
		errMsg := "hostfuncs: failed to read TCP request from Guest memory"
		slog.ErrorContext(ctx, errMsg)
		stack[0] = hostWriteResponse(ctx, mod, TCPResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "internal"},
		})
		return
	}

	var request TCPRequestWire
	if err := json.Unmarshal(requestBytes, &request); err != nil {
		errMsg := fmt.Sprintf("hostfuncs: failed to unmarshal TCP request: %v", err)
		slog.ErrorContext(ctx, errMsg)
		stack[0] = hostWriteResponse(ctx, mod, TCPResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "internal"},
		})
		return
	}

	// Create a new context from the wire format, with parent ctx for cancellation.
	tcpCtx, cancel := createContextFromWire(ctx, request.Context)
	defer cancel() // Ensure context resources are released.

	// Apply timeout from request if specified
	if request.TimeoutMs > 0 {
		tcpCtx, cancel = context.WithTimeout(tcpCtx, time.Duration(request.TimeoutMs)*time.Millisecond)
		defer cancel()
	}

	// 1. Check capability for outbound TCP
	pluginName := mod.Name()
	if name, ok := PluginNameFromContext(ctx); ok {
		pluginName = name
	}

	if err := checker.Check(pluginName, "network", fmt.Sprintf("outbound:%s", request.Port)); err != nil {
		errMsg := fmt.Sprintf("permission denied: %v", err)
		slog.WarnContext(ctx, errMsg, "host", request.Host, "port", request.Port)
		stack[0] = hostWriteResponse(ctx, mod, TCPResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "capability"},
		})
		return
	}

	// 2. Validate input
	if request.Host == "" {
		errMsg := "host cannot be empty"
		slog.WarnContext(ctx, errMsg)
		stack[0] = hostWriteResponse(ctx, mod, TCPResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "config"},
		})
		return
	}

	// SSRF protection: Resolve hostname ONCE, validate IP, then use validated IP
	// This prevents DNS rebinding attacks where DNS changes between validation and connection
	validatedIP, err := resolveAndValidate(ctx, request.Host, pluginName, checker)
	if err != nil {
		errMsg := fmt.Sprintf("SSRF protection: %v", err)
		slog.WarnContext(ctx, errMsg, "host", request.Host, "port", request.Port)
		stack[0] = hostWriteResponse(ctx, mod, TCPResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "ssrf_protection"},
		})
		return
	}

	if request.Port == "" {
		errMsg := "port cannot be empty"
		slog.WarnContext(ctx, errMsg)
		stack[0] = hostWriteResponse(ctx, mod, TCPResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "config"},
		})
		return
	}

	// 3. Perform TCP connection test using validated IP
	start := time.Now()
	response, err := performTCPConnect(tcpCtx, validatedIP, request.Port, request.TLS, request.Host)
	responseTime := time.Since(start).Milliseconds()

	if err != nil {
		errMsg := fmt.Sprintf("TCP connection failed: %v", err)
		slog.ErrorContext(ctx, errMsg, "host", request.Host, "port", request.Port)
		stack[0] = hostWriteResponse(ctx, mod, TCPResponseWire{
			Error: toErrorDetail(err),
		})
		return
	}

	// Add response time to result
	response.ResponseTimeMs = responseTime

	// 4. Write success response
	stack[0] = hostWriteResponse(ctx, mod, *response)
}

// performTCPConnect executes the actual TCP connection test
// validatedIP is the pre-resolved and validated IP address to connect to
// originalHost is the original hostname (used for TLS SNI and logging)
func performTCPConnect(ctx context.Context, validatedIP, port string, useTLS bool, originalHost string) (*TCPResponseWire, error) {
	// Connect to the validated IP address, not the hostname
	// This prevents DNS rebinding attacks
	address := net.JoinHostPort(validatedIP, port)

	response := &TCPResponseWire{
		Connected: false,
		// Use original hostname in address field for user-friendliness
		// (actual connection uses validated IP for security)
		Address: net.JoinHostPort(originalHost, port),
	}

	// Create dialer with context
	dialer := &net.Dialer{}

	if !useTLS {
		// Plain TCP connection
		conn, err := dialer.DialContext(ctx, "tcp", address)
		if err != nil {
			return nil, fmt.Errorf("connection failed: %w", err)
		}
		defer func() {
			_ = conn.Close() // Best-effort cleanup
		}()

		response.Connected = true
		response.RemoteAddr = conn.RemoteAddr().String()
		response.LocalAddr = conn.LocalAddr().String()

		return response, nil
	}

	// TLS connection
	tlsConfig := &tls.Config{
		// Use original hostname for SNI (Server Name Indication), not the IP
		ServerName: originalHost,
		MinVersion: tls.VersionTLS12,
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", address, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("TLS connection failed: %w", err)
	}
	defer func() {
		_ = conn.Close() // Best-effort cleanup
	}()

	// Get TLS connection state
	state := conn.ConnectionState()

	response.Connected = true
	response.RemoteAddr = conn.RemoteAddr().String()
	response.LocalAddr = conn.LocalAddr().String()
	response.TLS = true
	response.TLSVersion = tlsVersionString(state.Version)
	response.TLSCipherSuite = tls.CipherSuiteName(state.CipherSuite)
	response.TLSServerName = state.ServerName

	// Certificate info (basic)
	if len(state.PeerCertificates) > 0 {
		cert := state.PeerCertificates[0]
		response.TLSCertSubject = cert.Subject.String()
		response.TLSCertIssuer = cert.Issuer.String()
		response.TLSCertNotAfter = &cert.NotAfter
	}

	return response, nil
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
