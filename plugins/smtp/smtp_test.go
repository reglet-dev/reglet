package main

import (
	"context"
	"errors"
	"testing"
	"time"

	regletsdk "github.com/reglet-dev/reglet-sdk/go/application/config"
	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
)

type mockSMTPClient struct {
	ConnectFunc func(ctx context.Context, host, port string, timeout time.Duration, useTLS, useStartTLS bool) (*ports.SMTPConnectResult, error)
}

func (m *mockSMTPClient) Connect(ctx context.Context, host, port string, timeout time.Duration, useTLS, useStartTLS bool) (*ports.SMTPConnectResult, error) {
	return m.ConnectFunc(ctx, host, port, timeout, useTLS, useStartTLS)
}

func TestSMTPPlugin_Check_Success(t *testing.T) {
	mockDialer := func(ctx context.Context, host, port string, timeout time.Duration, useTLS, useStartTLS bool) (*ports.SMTPConnectResult, error) {
		return &ports.SMTPConnectResult{
			Connected:    true,
			ResponseTime: 10 * time.Millisecond,
			Banner:       "220 smtp.example.com ESMTP",
		}, nil
	}

	plugin := &smtpPlugin{client: &mockSMTPClient{ConnectFunc: mockDialer}}
	config := regletsdk.Config{
		"host": "smtp.example.com",
		"port": 25,
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if !evidence.IsSuccess() {
		t.Errorf("Expected status success, got %v", evidence.Status)
	}

	if evidence.Data["banner"] != "220 smtp.example.com ESMTP" {
		t.Errorf("Expected banner to be set, got %v", evidence.Data["banner"])
	}
}

func TestSMTPPlugin_Check_ConnectionRefused(t *testing.T) {
	mockDialer := func(ctx context.Context, host, port string, timeout time.Duration, useTLS, useStartTLS bool) (*ports.SMTPConnectResult, error) {
		return nil, errors.New("connection refused")
	}

	plugin := &smtpPlugin{client: &mockSMTPClient{ConnectFunc: mockDialer}}
	config := regletsdk.Config{
		"host": "localhost",
		"port": 25,
	}

	evidence, err := plugin.Check(context.Background(), config)

	// SDK returns error on connection failure
	if err == nil && evidence.IsSuccess() {
		t.Fatalf("Expected error or failure status, got success and nil error")
	}

	if evidence.IsSuccess() {
		t.Errorf("Expected status failure/error, got success")
	}
	if evidence.Error != nil && evidence.Error.Type != "network" {
		t.Errorf("Expected network error, got %v", evidence.Error)
	}
}

func TestSMTPPlugin_Check_WithTLS(t *testing.T) {
	mockDialer := func(ctx context.Context, host, port string, timeout time.Duration, useTLS, useStartTLS bool) (*ports.SMTPConnectResult, error) {
		return &ports.SMTPConnectResult{
			Connected:      true,
			ResponseTime:   20 * time.Millisecond,
			Banner:         "220 smtp.example.com ESMTP",
			TLSEnabled:     true,
			TLSVersion:     "TLS 1.3",
			TLSCipherSuite: "TLS_AES_128_GCM_SHA256",
			TLSServerName:  "smtp.example.com",
		}, nil
	}

	plugin := &smtpPlugin{client: &mockSMTPClient{ConnectFunc: mockDialer}}
	config := regletsdk.Config{
		"host":    "smtp.example.com",
		"port":    465,
		"use_tls": true,
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if !evidence.IsSuccess() {
		t.Errorf("Expected status success, got %v", evidence.Status)
	}

	// Note: Evidence data keys might depend on how RunSMTPCheck maps ports.SMTPConnectResult.
	// RunSMTPCheck uses "tls_version", etc.
}

func TestSMTPPlugin_Check_WithStartTLS(t *testing.T) {
	mockDialer := func(ctx context.Context, host, port string, timeout time.Duration, useTLS, useStartTLS bool) (*ports.SMTPConnectResult, error) {
		if !useStartTLS {
			t.Errorf("Expected StartTLS to be true")
		}
		return &ports.SMTPConnectResult{
			Connected:      true,
			ResponseTime:   15 * time.Millisecond,
			Banner:         "220 smtp.example.com ESMTP",
			TLSEnabled:     true,
			TLSVersion:     "TLS 1.2",
			TLSCipherSuite: "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
		}, nil
	}

	plugin := &smtpPlugin{client: &mockSMTPClient{ConnectFunc: mockDialer}}
	config := regletsdk.Config{
		"host":         "smtp.example.com",
		"port":         587,
		"use_starttls": true,
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if !evidence.IsSuccess() {
		t.Errorf("Expected status success, got %v", evidence.Status)
	}
}

func TestSMTPPlugin_Check_InvalidConfig(t *testing.T) {
	plugin := &smtpPlugin{
		client: &mockSMTPClient{
			ConnectFunc: func(ctx context.Context, host, port string, timeout time.Duration, useTLS, useStartTLS bool) (*ports.SMTPConnectResult, error) {
				return nil, nil
			},
		},
	}
	config := regletsdk.Config{
		// Missing required "host" field
		"port": 25,
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if evidence.IsSuccess() {
		t.Errorf("Expected status failure/error for invalid config")
	}
	if evidence.Error == nil || evidence.Error.Type != "config" {
		t.Errorf("Expected config error, got %v", evidence.Error)
	}
}
