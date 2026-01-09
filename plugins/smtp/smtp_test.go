//go:build wasip1

package main

import (
	"context"
	"errors"
	"testing"

	regletsdk "github.com/reglet-dev/reglet/sdk"
	regletnet "github.com/reglet-dev/reglet/sdk/net"
)

func TestSMTPPlugin_Check_Success(t *testing.T) {
	mockDialer := func(ctx context.Context, host, port string, timeoutMs int, useTLS bool, useStartTLS bool) (*regletnet.SMTPConnectResult, error) {
		return &regletnet.SMTPConnectResult{
			Connected:      true,
			Address:        host + ":" + port,
			ResponseTimeMs: 10,
			Banner:         "220 smtp.example.com ESMTP",
		}, nil
	}

	plugin := &smtpPlugin{DialSMTP: mockDialer}
	config := regletsdk.Config{
		"host": "smtp.example.com",
		"port": "25",
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if !evidence.Status {
		t.Errorf("Expected status true, got false")
	}

	if evidence.Data["banner"] != "220 smtp.example.com ESMTP" {
		t.Errorf("Expected banner to be set, got %v", evidence.Data["banner"])
	}
}

func TestSMTPPlugin_Check_ConnectionRefused(t *testing.T) {
	mockDialer := func(ctx context.Context, host, port string, timeoutMs int, useTLS bool, useStartTLS bool) (*regletnet.SMTPConnectResult, error) {
		return nil, errors.New("connection refused")
	}

	plugin := &smtpPlugin{DialSMTP: mockDialer}
	config := regletsdk.Config{
		"host": "localhost",
		"port": "25",
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

func TestSMTPPlugin_Check_WithTLS(t *testing.T) {
	mockDialer := func(ctx context.Context, host, port string, timeoutMs int, useTLS bool, useStartTLS bool) (*regletnet.SMTPConnectResult, error) {
		return &regletnet.SMTPConnectResult{
			Connected:      true,
			Address:        host + ":" + port,
			ResponseTimeMs: 20,
			Banner:         "220 smtp.example.com ESMTP",
			TLS:            true,
			TLSVersion:     "TLS 1.3",
			TLSCipherSuite: "TLS_AES_128_GCM_SHA256",
			TLSServerName:  "smtp.example.com",
		}, nil
	}

	plugin := &smtpPlugin{DialSMTP: mockDialer}
	config := regletsdk.Config{
		"host": "smtp.example.com",
		"port": "465",
		"tls":  true,
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if !evidence.Status {
		t.Errorf("Expected status true, got false")
	}

	if evidence.Data["tls"] != true {
		t.Errorf("Expected TLS to be true")
	}

	if evidence.Data["tls_version"] != "TLS 1.3" {
		t.Errorf("Expected TLS version 1.3, got %v", evidence.Data["tls_version"])
	}
}

func TestSMTPPlugin_Check_WithStartTLS(t *testing.T) {
	mockDialer := func(ctx context.Context, host, port string, timeoutMs int, useTLS bool, useStartTLS bool) (*regletnet.SMTPConnectResult, error) {
		if !useStartTLS {
			t.Errorf("Expected StartTLS to be true")
		}
		return &regletnet.SMTPConnectResult{
			Connected:      true,
			Address:        host + ":" + port,
			ResponseTimeMs: 15,
			Banner:         "220 smtp.example.com ESMTP",
			TLS:            true,
			TLSVersion:     "TLS 1.2",
			TLSCipherSuite: "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
		}, nil
	}

	plugin := &smtpPlugin{DialSMTP: mockDialer}
	config := regletsdk.Config{
		"host":     "smtp.example.com",
		"port":     "587",
		"starttls": true,
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if !evidence.Status {
		t.Errorf("Expected status true, got false")
	}

	if evidence.Data["tls"] != true {
		t.Errorf("Expected TLS to be true after STARTTLS")
	}
}

func TestSMTPPlugin_Check_InvalidConfig(t *testing.T) {
	plugin := &smtpPlugin{
		DialSMTP: func(ctx context.Context, host, port string, timeoutMs int, useTLS bool, useStartTLS bool) (*regletnet.SMTPConnectResult, error) {
			return nil, nil
		},
	}
	config := regletsdk.Config{
		// Missing required "host" field
		"port": "25",
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if evidence.Status {
		t.Errorf("Expected status false for invalid config")
	}
	if evidence.Error == nil || evidence.Error.Type != "config" {
		t.Errorf("Expected config error, got %v", evidence.Error)
	}
}
