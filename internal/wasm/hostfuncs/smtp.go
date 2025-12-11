package hostfuncs

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

	"github.com/tetratelabs/wazero/api"
)

// SMTPConnect performs SMTP connection tests on behalf of the plugin.
// It receives a packed uint64 (ptr+len) pointing to a JSON-encoded SMTPRequestWire.
// It returns a packed uint64 (ptr+len) pointing to a JSON-encoded SMTPResponseWire.
func SMTPConnect(ctx context.Context, mod api.Module, stack []uint64, checker *CapabilityChecker) {
	// Stack contains a single uint64 which is packed ptr+len of the request.
	requestPacked := stack[0]
	ptr, length := unpackPtrLen(requestPacked)

	requestBytes, ok := mod.Memory().Read(ptr, length)
	if !ok {
		// Critical error, Host could not read Guest memory.
		errMsg := "hostfuncs: failed to read SMTP request from Guest memory"
		slog.ErrorContext(ctx, errMsg)
		stack[0] = hostWriteResponse(ctx, mod, SMTPResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "internal"},
		})
		return
	}

	var request SMTPRequestWire
	if err := json.Unmarshal(requestBytes, &request); err != nil {
		errMsg := fmt.Sprintf("hostfuncs: failed to unmarshal SMTP request: %v", err)
		slog.ErrorContext(ctx, errMsg)
		stack[0] = hostWriteResponse(ctx, mod, SMTPResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "internal"},
		})
		return
	}

	// Create a new context from the wire format, with parent ctx for cancellation.
	smtpCtx, cancel := createContextFromWire(ctx, request.Context)
	defer cancel() // Ensure context resources are released.

	// Apply timeout from request if specified
	if request.TimeoutMs > 0 {
		smtpCtx, cancel = context.WithTimeout(smtpCtx, time.Duration(request.TimeoutMs)*time.Millisecond)
		defer cancel()
	}

	// 1. Check capability for outbound SMTP
	pluginName := mod.Name()
	if name, ok := PluginNameFromContext(ctx); ok {
		pluginName = name
	}

	if err := checker.Check(pluginName, "network", fmt.Sprintf("outbound:%s", request.Port)); err != nil {
		errMsg := fmt.Sprintf("permission denied: %v", err)
		slog.WarnContext(ctx, errMsg, "host", request.Host, "port", request.Port)
		stack[0] = hostWriteResponse(ctx, mod, SMTPResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "capability"},
		})
		return
	}

	// 2. Validate input
	if request.Host == "" {
		errMsg := "host cannot be empty"
		slog.WarnContext(ctx, errMsg)
		stack[0] = hostWriteResponse(ctx, mod, SMTPResponseWire{
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
		stack[0] = hostWriteResponse(ctx, mod, SMTPResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "ssrf_protection"},
		})
		return
	}

	if request.Port == "" {
		errMsg := "port cannot be empty"
		slog.WarnContext(ctx, errMsg)
		stack[0] = hostWriteResponse(ctx, mod, SMTPResponseWire{
			Error: &ErrorDetail{Message: errMsg, Type: "config"},
		})
		return
	}

	// 3. Perform SMTP connection test using validated IP
	start := time.Now()
	response, err := performSMTPConnect(smtpCtx, validatedIP, request.Port, request.TLS, request.StartTLS, request.Host)
	responseTime := time.Since(start).Milliseconds()

	if err != nil {
		errMsg := fmt.Sprintf("SMTP connection failed: %v", err)
		slog.ErrorContext(ctx, errMsg, "host", request.Host, "port", request.Port)
		stack[0] = hostWriteResponse(ctx, mod, SMTPResponseWire{
			Error: toErrorDetail(err),
		})
		return
	}

	// Add response time to result
	response.ResponseTimeMs = responseTime

	// 4. Write success response
	stack[0] = hostWriteResponse(ctx, mod, *response)
}

// performSMTPConnect executes the actual SMTP connection test
// validatedIP is the pre-resolved and validated IP address to connect to
// originalHost is the original hostname (used for TLS SNI and SMTP HELO)
func performSMTPConnect(ctx context.Context, validatedIP, port string, useTLS bool, useStartTLS bool, originalHost string) (*SMTPResponseWire, error) {
	// Connect to the validated IP address, not the hostname
	// This prevents DNS rebinding attacks
	address := net.JoinHostPort(validatedIP, port)

	response := &SMTPResponseWire{
		Connected: false,
		// Use original hostname in address field for user-friendliness
		// (actual connection uses validated IP for security)
		Address: net.JoinHostPort(originalHost, port),
	}

	if useTLS {
		// Direct TLS connection (SMTPS on port 465)
		tlsConfig := &tls.Config{
			ServerName: originalHost,
			MinVersion: tls.VersionTLS12,
		}

		conn, err := tls.Dial("tcp", address, tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("TLS connection failed: %w", err)
		}
		defer func() {
			_ = conn.Close() // Best-effort cleanup
		}()

		// Read banner using textproto
		tp := textproto.NewReader(bufio.NewReader(conn))
		code, msg, err := tp.ReadResponse(220)
		if err != nil {
			return nil, fmt.Errorf("failed to read SMTP banner: %w", err)
		}

		banner := fmt.Sprintf("%d %s", code, msg)

		// Get TLS connection state
		state := conn.ConnectionState()

		response.Connected = true
		response.Banner = strings.TrimSpace(banner)
		response.TLS = true
		response.TLSVersion = tlsVersionString(state.Version)
		response.TLSCipherSuite = tls.CipherSuiteName(state.CipherSuite)
		response.TLSServerName = state.ServerName

		return response, nil
	}

	// Plain connection (possibly with STARTTLS)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer func() {
		_ = conn.Close() // Best-effort cleanup
	}()

	// Read banner using textproto
	tp := textproto.NewReader(bufio.NewReader(conn))
	code, msg, err := tp.ReadResponse(220)
	if err != nil {
		return nil, fmt.Errorf("failed to read SMTP banner: %w", err)
	}

	banner := fmt.Sprintf("%d %s", code, msg)

	response.Connected = true
	response.Banner = strings.TrimSpace(banner)

	if useStartTLS {
		// For STARTTLS, we need to use the SMTP client
		client, err := smtp.NewClient(conn, originalHost)
		if err != nil {
			return nil, fmt.Errorf("SMTP client creation failed: %w", err)
		}
		defer func() {
			_ = client.Quit() // Best-effort cleanup
		}()

		// Upgrade to TLS via STARTTLS
		tlsConfig := &tls.Config{
			ServerName: originalHost,
			MinVersion: tls.VersionTLS12,
		}

		if err := client.StartTLS(tlsConfig); err != nil {
			return nil, fmt.Errorf("STARTTLS failed: %w", err)
		}

		// Get TLS connection state after upgrade
		state, ok := client.TLSConnectionState()
		if !ok {
			return nil, fmt.Errorf("failed to get TLS state after STARTTLS")
		}

		response.TLS = true
		response.TLSVersion = tlsVersionString(state.Version)
		response.TLSCipherSuite = tls.CipherSuiteName(state.CipherSuite)
		response.TLSServerName = state.ServerName
	}

	return response, nil
}
