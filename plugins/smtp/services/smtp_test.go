package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
	"github.com/reglet-dev/reglet/plugins/smtp/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSMTPClient struct {
	ConnectFunc func(ctx context.Context, host, port string, timeout time.Duration, useTLS, useStartTLS bool) (*ports.SMTPConnectResult, error)
}

func (m *mockSMTPClient) Connect(ctx context.Context, host, port string, timeout time.Duration, useTLS, useStartTLS bool) (*ports.SMTPConnectResult, error) {
	return m.ConnectFunc(ctx, host, port, timeout, useTLS, useStartTLS)
}

func TestSMTPService_Connect_Success(t *testing.T) {
	mockClient := &mockSMTPClient{
		ConnectFunc: func(ctx context.Context, host, port string, timeout time.Duration, useTLS, useStartTLS bool) (*ports.SMTPConnectResult, error) {
			return &ports.SMTPConnectResult{
				Connected:    true,
				ResponseTime: 10 * time.Millisecond,
				Banner:       "220 smtp.example.com ESMTP",
			}, nil
		},
	}

	svc := &SMTPService{}
	cfg := &core.SMTPConfig{
		Host: "smtp.example.com",
		Port: 25,
	}
	req := &plugin.Request{
		Client: mockClient,
		Config: cfg,
	}

	result, err := svc.ConnectHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.ResultStatusSuccess, result.Status)
	assert.Contains(t, result.Data["banner"], "220 smtp.example.com ESMTP")
}

func TestSMTPService_Connect_Fail(t *testing.T) {
	mockClient := &mockSMTPClient{
		ConnectFunc: func(ctx context.Context, host, port string, timeout time.Duration, useTLS, useStartTLS bool) (*ports.SMTPConnectResult, error) {
			return nil, errors.New("connection failed")
		},
	}

	svc := &SMTPService{}
	cfg := &core.SMTPConfig{
		Host: "smtp.example.com",
		Port: 25,
	}
	req := &plugin.Request{
		Client: mockClient,
		Config: cfg,
	}

	result, err := svc.ConnectHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.ResultStatusFailure, result.Status)
}

func TestSMTPService_Connect_WithTLS(t *testing.T) {
	mockClient := &mockSMTPClient{
		ConnectFunc: func(ctx context.Context, host, port string, timeout time.Duration, useTLS, useStartTLS bool) (*ports.SMTPConnectResult, error) {
			assert.True(t, useTLS)
			return &ports.SMTPConnectResult{
				Connected:      true,
				TLSEnabled:     true,
				TLSVersion:     "TLS 1.3",
				TLSCipherSuite: "TLS_AES_128_GCM_SHA256",
			}, nil
		},
	}

	svc := &SMTPService{}
	cfg := &core.SMTPConfig{
		Host:   "smtp.example.com",
		Port:   465,
		UseTLS: true,
	}
	req := &plugin.Request{
		Client: mockClient,
		Config: cfg,
	}

	result, err := svc.ConnectHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.ResultStatusSuccess, result.Status)
	assert.Equal(t, "TLS 1.3", result.Data["tls_version"])
}

func TestSMTPService_Connect_WithStartTLS(t *testing.T) {
	mockClient := &mockSMTPClient{
		ConnectFunc: func(ctx context.Context, host, port string, timeout time.Duration, useTLS, useStartTLS bool) (*ports.SMTPConnectResult, error) {
			assert.True(t, useStartTLS)
			return &ports.SMTPConnectResult{
				Connected:  true,
				TLSEnabled: true,
				TLSVersion: "TLS 1.2",
			}, nil
		},
	}

	svc := &SMTPService{}
	cfg := &core.SMTPConfig{
		Host:     "smtp.example.com",
		Port:     587,
		StartTLS: true,
	}
	req := &plugin.Request{
		Client: mockClient,
		Config: cfg,
	}

	result, err := svc.ConnectHandler(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, entities.ResultStatusSuccess, result.Status)
}
