//go:build wasip1

// Package net
package net

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/whiskeyjimbo/reglet/sdk/internal/abi"
	_ "github.com/whiskeyjimbo/reglet/sdk/log" // Initialize WASM logging handler
)

// host_smtp_connect calls the host function to establish an SMTP connection
// Using the new wire format with packed ptr+len
//
//go:wasmimport reglet_host smtp_connect
func host_smtp_connect(requestPacked uint64) uint64

// SMTPConnectResult contains the result of an SMTP connection test
type SMTPConnectResult struct {
	Connected      bool
	Address        string
	Banner         string
	ResponseTimeMs int64
	TLS            bool
	TLSVersion     string
	TLSCipherSuite string
	TLSServerName  string
}

// DialSMTP connects to the given SMTP host and port via the host runtime.
// It uses the wire format protocol for communication with the host.
func DialSMTP(ctx context.Context, host, port string, timeoutMs int, useTLS bool, useStartTLS bool) (*SMTPConnectResult, error) {
	// Build request using wire format
	request := SMTPRequestWire{
		Context:   createContextWireFormat(ctx),
		Host:      host,
		Port:      port,
		TimeoutMs: timeoutMs,
		TLS:       useTLS,
		StartTLS:  useStartTLS,
	}

	// Marshal request to JSON
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal SMTP request: %w", err)
	}

	// Allocate and write request
	requestPacked := abi.PtrFromBytes(requestBytes)
	defer abi.DeallocatePacked(requestPacked)

	// Call host function
	responsePacked := host_smtp_connect(requestPacked)

	// Read response
	responseBytes := abi.BytesFromPtr(responsePacked)
	defer abi.DeallocatePacked(responsePacked)

	// Unmarshal response
	var response SMTPResponseWire
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal SMTP response: %w", err)
	}

	// Check for error in response
	if response.Error != nil {
		return nil, fmt.Errorf("%s: %s", response.Error.Type, response.Error.Message)
	}

	// Convert to result struct
	result := &SMTPConnectResult{
		Connected:      response.Connected,
		Address:        response.Address,
		Banner:         response.Banner,
		ResponseTimeMs: response.ResponseTimeMs,
		TLS:            response.TLS,
		TLSVersion:     response.TLSVersion,
		TLSCipherSuite: response.TLSCipherSuite,
		TLSServerName:  response.TLSServerName,
	}

	return result, nil
}
