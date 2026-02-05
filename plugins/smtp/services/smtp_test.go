package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/reglet-dev/reglet-sdk/application/plugin"
	"github.com/reglet-dev/reglet-sdk/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSMTPClient struct {
	ConnectFunc func(ctx context.Context, host, port string, timeout time.Duration, useTLS, useStartTLS bool) (*ports.SMTPConnectResult, error)
}

func (m *mockSMTPClient) Connect(ctx context.Context, host, port string, timeout time.Duration, useTLS, useStartTLS bool) (*ports.SMTPConnectResult, error) {
	return m.ConnectFunc(ctx, host, port, timeout, useTLS, useStartTLS)
}

func TestExamples(t *testing.T) {
	t.Skip("Requires mock injection setup for examples")
}

func TestSMTPService_Connect_Success(t *testing.T) {
	mockClient := &mockSMTPClient{
		ConnectFunc: func(ctx context.Context, host, port string, timeout time.Duration, useTLS, useStartTLS bool) (*ports.SMTPConnectResult, error) {
			return &ports.SMTPConnectResult{
				Connected:    true,
				ResponseTime: 10 * time.Millisecond,
				Banner:       "220 smtp.example.com ESMTP",
				Extensions:   []string{"STARTTLS", "AUTH LOGIN PLAIN", "SIZE 35882577"},
				SupportsAuth: true,
			}, nil
		},
	}

	svc := &SMTPService{}
	input := &ConnectInput{
		Host: "smtp.example.com",
		Port: 25,
	}

	ctx := context.Background()
	ctx = plugin.WithClient(ctx, ports.SMTPClient(mockClient))

	output, err := svc.ConnectHandler(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, "smtp.example.com", output.Host)
	assert.Equal(t, 25, output.Port)
	assert.Contains(t, output.Banner, "220 smtp.example.com ESMTP")
	assert.True(t, output.SupportsAuth)
	assert.Contains(t, output.Extensions, "STARTTLS")
}

func TestSMTPService_Connect_Fail(t *testing.T) {
	mockClient := &mockSMTPClient{
		ConnectFunc: func(ctx context.Context, host, port string, timeout time.Duration, useTLS, useStartTLS bool) (*ports.SMTPConnectResult, error) {
			return nil, errors.New("connection failed")
		},
	}

	svc := &SMTPService{}
	input := &ConnectInput{
		Host: "smtp.example.com",
		Port: 25,
	}

	ctx := context.Background()
	ctx = plugin.WithClient(ctx, ports.SMTPClient(mockClient))

	output, err := svc.ConnectHandler(ctx, input)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "connection failed")
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
	input := &ConnectInput{
		Host:   "smtp.example.com",
		Port:   465,
		UseTLS: true,
	}

	ctx := context.Background()
	ctx = plugin.WithClient(ctx, ports.SMTPClient(mockClient))

	output, err := svc.ConnectHandler(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, "TLS 1.3", output.TLSVersion)
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
	input := &ConnectInput{
		Host:        "smtp.example.com",
		Port:        587,
		UseSTARTTLS: true,
	}

	ctx := context.Background()
	ctx = plugin.WithClient(ctx, ports.SMTPClient(mockClient))

	output, err := svc.ConnectHandler(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, "TLS 1.2", output.TLSVersion)
}
