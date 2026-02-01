package services

import (
	"context"
	"testing"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
	"github.com/reglet-dev/reglet/plugins/dns/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockDNSResolver implements ports.DNSResolver
type MockDNSResolver struct {
	IPs    map[string][]string
	CNAMEs map[string]string
	MXs    map[string][]ports.MXRecord
	TXTs   map[string][]string
	Err    error
}

func (m *MockDNSResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if recs, ok := m.IPs[host]; ok {
		return recs, nil
	}
	return nil, nil
}

func (m *MockDNSResolver) LookupCNAME(ctx context.Context, host string) (string, error) {
	if m.Err != nil {
		return "", m.Err
	}
	if val, ok := m.CNAMEs[host]; ok {
		return val, nil
	}
	return "", nil
}

func (m *MockDNSResolver) LookupMX(ctx context.Context, domain string) ([]ports.MXRecord, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if recs, ok := m.MXs[domain]; ok {
		return recs, nil
	}
	return nil, nil
}

func (m *MockDNSResolver) LookupTXT(ctx context.Context, domain string) ([]string, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if recs, ok := m.TXTs[domain]; ok {
		return recs, nil
	}
	return nil, nil
}

func (m *MockDNSResolver) LookupNS(ctx context.Context, domain string) ([]string, error) {
	return nil, nil // Not implemented for mock
}

func TestDNSService_Resolve_Success(t *testing.T) {
	svc := &DNSService{}
	mockResolver := &MockDNSResolver{
		IPs: map[string][]string{
			"example.com": {"93.184.216.34"},
		},
	}
	cfg := &core.DNSConfig{
		Hostname: "example.com",
	}
	req := &plugin.Request{
		Client: mockResolver,
		Config: cfg,
	}

	result, err := svc.ResolveHandler(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, entities.ResultStatusSuccess, result.Status)
	assert.Contains(t, result.Data["records"], "93.184.216.34")
}

func TestDNSService_ValidateA_Success(t *testing.T) {
	svc := &DNSService{}
	mockResolver := &MockDNSResolver{
		IPs: map[string][]string{
			"example.com": {"93.184.216.34"},
		},
	}
	cfg := &core.DNSConfig{
		Hostname:       "example.com",
		ExpectedValues: []string{"93.184.216.34"},
	}
	req := &plugin.Request{
		Client: mockResolver,
		Config: cfg,
	}

	result, err := svc.ValidateAHandler(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, entities.ResultStatusSuccess, result.Status)
}

func TestDNSService_ValidateA_Fail(t *testing.T) {
	svc := &DNSService{}
	mockResolver := &MockDNSResolver{
		IPs: map[string][]string{
			"example.com": {"1.1.1.1"},
		},
	}
	cfg := &core.DNSConfig{
		Hostname:       "example.com",
		ExpectedValues: []string{"93.184.216.34"},
	}
	req := &plugin.Request{
		Client: mockResolver,
		Config: cfg,
	}

	result, err := svc.ValidateAHandler(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, entities.ResultStatusFailure, result.Status)
	assert.Contains(t, result.Message, "Missing expected records")
}

func TestDNSService_ValidateMX_Success(t *testing.T) {
	svc := &DNSService{}
	mockResolver := &MockDNSResolver{
		MXs: map[string][]ports.MXRecord{
			"example.com": {{Host: "mail.example.com.", Pref: 10}},
		},
	}
	cfg := &core.DNSConfig{
		Hostname:       "example.com",
		RecordType:     "MX",
		ExpectedValues: []string{"mail.example.com"},
	}
	req := &plugin.Request{
		Client: mockResolver,
		Config: cfg,
	}

	result, err := svc.ValidateMXHandler(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, entities.ResultStatusSuccess, result.Status)
}
