package services

import (
	"context"
	"errors"
	"testing"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
	"github.com/reglet-dev/reglet/plugins/dns/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDNSResolver implements ports.DNSResolver for testing.
type mockDNSResolver struct{}

func (m *mockDNSResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	switch host {
	case "example.com":
		return []string{"93.184.216.34"}, nil
	case "this-domain-does-not-exist.invalid":
		return nil, errors.New("DNS lookup failed: NXDOMAIN")
	default:
		return []string{"127.0.0.1"}, nil
	}
}

func (m *mockDNSResolver) LookupMX(ctx context.Context, host string) ([]ports.MXRecord, error) {
	if host == "example.com" {
		return []ports.MXRecord{
			{Host: "mail.example.com.", Pref: 10},
			{Host: "mail2.example.com.", Pref: 20},
		}, nil
	}
	return nil, errors.New("no MX records")
}

func (m *mockDNSResolver) LookupTXT(ctx context.Context, host string) ([]string, error) {
	if host == "example.com" {
		return []string{"v=spf1 include:_spf.example.com ~all"}, nil
	}
	return nil, errors.New("no TXT records")
}

func (m *mockDNSResolver) LookupCNAME(ctx context.Context, host string) (string, error) {
	return "www.example.com.", nil
}

func (m *mockDNSResolver) LookupNS(ctx context.Context, host string) ([]string, error) {
	return []string{"ns1.example.com.", "ns2.example.com."}, nil
}

// TestExamples runs auto-generated tests from registered examples.
// This ensures documentation examples stay in sync with implementation.
func TestExamples(t *testing.T) {
	plugin.GenerateExampleTests(t, core.Plugin, &mockDNSResolver{})
}

// TestResolve_Handler tests the handler directly with typed input/output.
func TestResolve_Handler(t *testing.T) {
	svc := &DNSService{}

	ctx := context.Background()
	ctx = plugin.WithClient(ctx, &mockDNSResolver{})

	out, err := svc.ResolveHandler(ctx, &ResolveInput{
		Hostname:   "example.com",
		RecordType: "A",
	})

	require.NoError(t, err)
	assert.Equal(t, "example.com", out.Hostname)
	assert.Equal(t, "A", out.RecordType)
	assert.Contains(t, out.Records, "93.184.216.34")
}

// TestResolve_DefaultRecordType verifies A is used when not specified.
func TestResolve_DefaultRecordType(t *testing.T) {
	svc := &DNSService{}

	ctx := context.Background()
	ctx = plugin.WithClient(ctx, &mockDNSResolver{})

	out, err := svc.ResolveHandler(ctx, &ResolveInput{
		Hostname: "example.com",
		// RecordType not specified
	})

	require.NoError(t, err)
	assert.Equal(t, "A", out.RecordType)
}

// TestResolve_UnsupportedType verifies error for invalid record type.
func TestResolve_UnsupportedType(t *testing.T) {
	svc := &DNSService{}

	ctx := context.Background()
	ctx = plugin.WithClient(ctx, &mockDNSResolver{})

	_, err := svc.ResolveHandler(ctx, &ResolveInput{
		Hostname:   "example.com",
		RecordType: "INVALID",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported record type")
}
