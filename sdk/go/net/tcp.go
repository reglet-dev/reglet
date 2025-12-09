//go:build wasip1

// Package net
package net

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/whiskeyjimbo/reglet/sdk/internal/abi"
	_ "github.com/whiskeyjimbo/reglet/sdk/log" // Initialize WASM logging handler
)

// host_tcp_connect calls the host function to establish a TCP connection
// Using the new wire format with packed ptr+len
//
//go:wasmimport reglet_host tcp_connect
func host_tcp_connect(requestPacked uint64) uint64

// TCPConnectResult contains the result of a TCP connection test
type TCPConnectResult struct {
	Connected       bool
	Address         string
	RemoteAddr      string
	LocalAddr       string
	ResponseTimeMs  int64
	TLS             bool
	TLSVersion      string
	TLSCipherSuite  string
	TLSServerName   string
	TLSCertSubject  string
	TLSCertIssuer   string
	TLSCertNotAfter *time.Time
}

// DialTCP connects to the given host and port via the host runtime.
// It uses the wire format protocol for communication with the host.
func DialTCP(ctx context.Context, host, port string, timeoutMs int, useTLS bool) (*TCPConnectResult, error) {
	// Build request using wire format
	request := TCPRequestWire{
		Context:   createContextWireFormat(ctx),
		Host:      host,
		Port:      port,
		TimeoutMs: timeoutMs,
		TLS:       useTLS,
	}

	// Marshal request to JSON
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal TCP request: %w", err)
	}

	// Allocate and write request
	requestPacked := abi.PtrFromBytes(requestBytes)
	defer abi.DeallocatePacked(requestPacked)

	// Call host function
	responsePacked := host_tcp_connect(requestPacked)

	// Read response
	responseBytes := abi.BytesFromPtr(responsePacked)
	defer abi.DeallocatePacked(responsePacked)

	// Unmarshal response
	var response TCPResponseWire
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal TCP response: %w", err)
	}

	// Check for error in response
	if response.Error != nil {
		return nil, fmt.Errorf("%s: %s", response.Error.Type, response.Error.Message)
	}

	// Convert to result struct
	result := &TCPConnectResult{
		Connected:       response.Connected,
		Address:         response.Address,
		RemoteAddr:      response.RemoteAddr,
		LocalAddr:       response.LocalAddr,
		ResponseTimeMs:  response.ResponseTimeMs,
		TLS:             response.TLS,
		TLSVersion:      response.TLSVersion,
		TLSCipherSuite:  response.TLSCipherSuite,
		TLSServerName:   response.TLSServerName,
		TLSCertSubject:  response.TLSCertSubject,
		TLSCertIssuer:   response.TLSCertIssuer,
		TLSCertNotAfter: response.TLSCertNotAfter,
	}

	return result, nil
}
