package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock TCPConnection
type mockTCPConnection struct {
	remoteAddr     string
	localAddr      string
	connected      bool
	isTLS          bool
	tlsVersion     string
	tlsCipherSuite string
	tlsServerName  string
	tlsCertSubject string
	tlsCertIssuer  string
	tlsNotAfter    *time.Time
}

func (m *mockTCPConnection) Close() error                { return nil }
func (m *mockTCPConnection) RemoteAddr() string          { return m.remoteAddr }
func (m *mockTCPConnection) IsConnected() bool           { return m.connected }
func (m *mockTCPConnection) LocalAddr() string           { return m.localAddr }
func (m *mockTCPConnection) IsTLS() bool                 { return m.isTLS }
func (m *mockTCPConnection) TLSVersion() string          { return m.tlsVersion }
func (m *mockTCPConnection) TLSCipherSuite() string      { return m.tlsCipherSuite }
func (m *mockTCPConnection) TLSServerName() string       { return m.tlsServerName }
func (m *mockTCPConnection) TLSCertSubject() string      { return m.tlsCertSubject }
func (m *mockTCPConnection) TLSCertIssuer() string       { return m.tlsCertIssuer }
func (m *mockTCPConnection) TLSCertNotAfter() *time.Time { return m.tlsNotAfter }

// Mock TCPDialer
type mockTCPDialer struct {
	DialSecureFunc func(ctx context.Context, address string, timeoutMs int, tls bool) (ports.TCPConnection, error)
}

func (m *mockTCPDialer) Dial(ctx context.Context, address string) (ports.TCPConnection, error) {
	return m.DialSecure(ctx, address, 0, false)
}

func (m *mockTCPDialer) DialWithTimeout(ctx context.Context, address string, timeoutMs int) (ports.TCPConnection, error) {
	return m.DialSecure(ctx, address, timeoutMs, false)
}

func (m *mockTCPDialer) DialSecure(ctx context.Context, address string, timeoutMs int, tls bool) (ports.TCPConnection, error) {
	if m.DialSecureFunc != nil {
		return m.DialSecureFunc(ctx, address, timeoutMs, tls)
	}
	return nil, errors.New("dial function not implemented")
}

func TestExamples(t *testing.T) {
	t.Skip("Requires mock injection setup for examples")
}

func TestTCPService_Connect_Success(t *testing.T) {
	mockDialer := &mockTCPDialer{
		DialSecureFunc: func(ctx context.Context, address string, timeoutMs int, tls bool) (ports.TCPConnection, error) {
			return &mockTCPConnection{
				connected:  true,
				remoteAddr: "1.2.3.4:80",
			}, nil
		},
	}

	svc := &TCPService{}
	input := &ConnectInput{
		Host: "example.com",
		Port: 80,
	}

	// Inject client into context
	ctx := context.Background()
	ctx = plugin.WithClient(ctx, ports.TCPDialer(mockDialer))

	output, err := svc.ConnectHandler(ctx, input)
	require.NoError(t, err)
	assert.True(t, output.Connected)
	assert.Equal(t, "example.com", output.Host)
}

func TestTCPService_Connect_Fail(t *testing.T) {
	mockDialer := &mockTCPDialer{
		DialSecureFunc: func(ctx context.Context, address string, timeoutMs int, tls bool) (ports.TCPConnection, error) {
			return nil, errors.New("connection failed")
		},
	}

	svc := &TCPService{}
	input := &ConnectInput{
		Host: "example.com",
		Port: 80,
	}

	ctx := context.Background()
	ctx = plugin.WithClient(ctx, ports.TCPDialer(mockDialer))

	output, err := svc.ConnectHandler(ctx, input)
	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "connection failed")
}

func TestTCPService_Connect_TLS_Version(t *testing.T) {
	mockDialer := &mockTCPDialer{
		DialSecureFunc: func(ctx context.Context, address string, timeoutMs int, tls bool) (ports.TCPConnection, error) {
			return &mockTCPConnection{
				connected:  true,
				isTLS:      true,
				tlsVersion: "TLS 1.2",
			}, nil
		},
	}

	svc := &TCPService{}
	input := &ConnectInput{
		Host: "example.com",
		Port: 443,
		TLS:  true,
	}

	ctx := context.Background()
	ctx = plugin.WithClient(ctx, ports.TCPDialer(mockDialer))

	output, err := svc.ConnectHandler(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, "TLS 1.2", output.TLSVersion)
}
