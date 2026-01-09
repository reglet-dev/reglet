//go:build wasip1

package main

import (
	"context"
	"errors"
	"testing"

	regletsdk "github.com/reglet-dev/reglet/sdk"
	regletnet "github.com/reglet-dev/reglet/sdk/net"
)

func TestTCPPlugin_Check_Success(t *testing.T) {
	mockDialer := func(ctx context.Context, host, port string, timeoutMs int, useTLS bool) (*regletnet.TCPConnectResult, error) {
		return &regletnet.TCPConnectResult{
			Connected:      true,
			Address:        host + ":" + port,
			ResponseTimeMs: 10,
			RemoteAddr:     "1.2.3.4:80",
		}, nil
	}

	plugin := &tcpPlugin{DialTCP: mockDialer}
	config := regletsdk.Config{
		"host": "example.com",
		"port": "80",
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if !evidence.Status {
		t.Errorf("Expected status true, got false")
	}
}

func TestTCPPlugin_Check_ConnectionRefused(t *testing.T) {
	mockDialer := func(ctx context.Context, host, port string, timeoutMs int, useTLS bool) (*regletnet.TCPConnectResult, error) {
		return nil, errors.New("connection refused")
	}

	plugin := &tcpPlugin{DialTCP: mockDialer}
	config := regletsdk.Config{
		"host": "localhost",
		"port": "12345",
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if evidence.Status {
		t.Errorf("Expected status false, got true")
	}
	if evidence.Error == nil || evidence.Error.Type != "network" {
		t.Errorf("Expected network error")
	}
}

func TestTCPPlugin_Check_TLS_Version_Pass(t *testing.T) {
	mockDialer := func(ctx context.Context, host, port string, timeoutMs int, useTLS bool) (*regletnet.TCPConnectResult, error) {
		return &regletnet.TCPConnectResult{
			Connected:  true,
			TLS:        true,
			TLSVersion: "TLS 1.3",
		}, nil
	}

	plugin := &tcpPlugin{DialTCP: mockDialer}
	config := regletsdk.Config{
		"host":                 "example.com",
		"port":                 "443",
		"tls":                  true,
		"expected_tls_version": "TLS 1.2",
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if evidence.Data["expectation_failed"] == true {
		t.Errorf("Expected expectation to pass")
	}
}

func TestTCPPlugin_Check_TLS_Version_Fail(t *testing.T) {
	mockDialer := func(ctx context.Context, host, port string, timeoutMs int, useTLS bool) (*regletnet.TCPConnectResult, error) {
		return &regletnet.TCPConnectResult{
			Connected:  true,
			TLS:        true,
			TLSVersion: "TLS 1.0",
		}, nil
	}

	plugin := &tcpPlugin{DialTCP: mockDialer}
	config := regletsdk.Config{
		"host":                 "example.com",
		"port":                 "443",
		"tls":                  true,
		"expected_tls_version": "TLS 1.2",
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if evidence.Data["expectation_failed"] != true {
		t.Errorf("Expected expectation_failed to be true")
	}
}
