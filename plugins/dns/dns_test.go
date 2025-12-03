package main

import (
	"context"
	"errors"
	"testing"

	regletsdk "github.com/whiskeyjimbo/reglet/sdk"
)

// Mock resolver implementation
type mockResolver struct {
	LookupHostFunc  func(ctx context.Context, host string, nameserver string) ([]string, error)
	LookupCNAMEFunc func(ctx context.Context, host string, nameserver string) (string, error)
	LookupMXFunc    func(ctx context.Context, host string, nameserver string) ([]string, error)
	LookupTXTFunc   func(ctx context.Context, host string, nameserver string) ([]string, error)
	LookupNSFunc    func(ctx context.Context, host string, nameserver string) ([]string, error)
}

func (m *mockResolver) LookupHost(ctx context.Context, host string, nameserver string) ([]string, error) {
	if m.LookupHostFunc != nil {
		return m.LookupHostFunc(ctx, host, nameserver)
	}
	return nil, nil
}

func (m *mockResolver) LookupCNAME(ctx context.Context, host string, nameserver string) (string, error) {
	if m.LookupCNAMEFunc != nil {
		return m.LookupCNAMEFunc(ctx, host, nameserver)
	}
	return "", nil
}

func (m *mockResolver) LookupMX(ctx context.Context, host string, nameserver string) ([]string, error) {
	if m.LookupMXFunc != nil {
		return m.LookupMXFunc(ctx, host, nameserver)
	}
	return nil, nil
}

func (m *mockResolver) LookupTXT(ctx context.Context, host string, nameserver string) ([]string, error) {
	if m.LookupTXTFunc != nil {
		return m.LookupTXTFunc(ctx, host, nameserver)
	}
	return nil, nil
}

func (m *mockResolver) LookupNS(ctx context.Context, host string, nameserver string) ([]string, error) {
	if m.LookupNSFunc != nil {
		return m.LookupNSFunc(ctx, host, nameserver)
	}
	return nil, nil
}

func TestDNSPlugin_Check_A_Record(t *testing.T) {
	mock := &mockResolver{
		LookupHostFunc: func(ctx context.Context, host string, nameserver string) ([]string, error) {
			if host == "example.com" {
				return []string{"93.184.216.34"}, nil
			}
			return nil, errors.New("host not found")
		},
	}

	plugin := &dnsPlugin{resolver: mock}
	config := regletsdk.Config{
		"hostname":    "example.com",
		"record_type": "A",
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if !evidence.Status {
		t.Errorf("Evidence status is false, expected true. Error: %v", evidence.Error)
	}

	data := evidence.Data
	if data["hostname"] != "example.com" {
		t.Errorf("Expected hostname 'example.com', got %v", data["hostname"])
	}
	records, ok := data["records"].([]string)
	if !ok {
		t.Fatalf("Expected records to be []string, got %T", data["records"])
	}
	if len(records) != 1 || records[0] != "93.184.216.34" {
		t.Errorf("Expected record '93.184.216.34', got %v", records)
	}
}

func TestDNSPlugin_Check_A_Record_FilterIPv6(t *testing.T) {
	mock := &mockResolver{
		LookupHostFunc: func(ctx context.Context, host string, nameserver string) ([]string, error) {
			// Return mixed IPv4 and IPv6
			return []string{"93.184.216.34", "2606:2800:220:1:248:1893:25c8:1946"}, nil
		},
	}

	plugin := &dnsPlugin{resolver: mock}
	config := regletsdk.Config{
		"hostname":    "example.com",
		"record_type": "A",
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	records, ok := evidence.Data["records"].([]string)
	if !ok {
		t.Fatalf("Expected records to be []string")
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 IPv4 record, got %d", len(records))
	}
	if records[0] != "93.184.216.34" {
		t.Errorf("Expected IPv4 record, got %s", records[0])
	}
}

func TestDNSPlugin_Check_AAAA_Record(t *testing.T) {
	mock := &mockResolver{
		LookupHostFunc: func(ctx context.Context, host string, nameserver string) ([]string, error) {
			// Return mixed IPv4 and IPv6
			return []string{"93.184.216.34", "2606:2800:220:1:248:1893:25c8:1946"}, nil
		},
	}

	plugin := &dnsPlugin{resolver: mock}
	config := regletsdk.Config{
		"hostname":    "example.com",
		"record_type": "AAAA",
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	records, ok := evidence.Data["records"].([]string)
	if !ok {
		t.Fatalf("Expected records to be []string")
	}

	if len(records) != 1 {
		t.Errorf("Expected 1 IPv6 record, got %d", len(records))
	}
	if records[0] != "2606:2800:220:1:248:1893:25c8:1946" {
		t.Errorf("Expected IPv6 record, got %s", records[0])
	}
}

func TestDNSPlugin_Check_MissingHostname(t *testing.T) {
	plugin := &dnsPlugin{resolver: &mockResolver{}}
	config := regletsdk.Config{} // Empty config

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}

	if evidence.Status {
		t.Error("Expected status false for missing hostname, got true")
	}
	if evidence.Error == nil || evidence.Error.Type != "config" {
		t.Errorf("Expected config error, got %v", evidence.Error)
	}
}

func TestDNSPlugin_Check_NetworkError(t *testing.T) {
	mock := &mockResolver{
		LookupHostFunc: func(ctx context.Context, host string, nameserver string) ([]string, error) {
			return nil, errors.New("simulated network failure")
		},
	}

	plugin := &dnsPlugin{resolver: mock}
	config := regletsdk.Config{"hostname": "error.com"}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned unexpected error: %v", err)
	}

	if evidence.Status {
		t.Error("Expected status false for network error, got true")
	}
	if evidence.Error == nil || evidence.Error.Type != "network" {
		t.Errorf("Expected network error, got %v", evidence.Error)
	}
}

func TestDNSPlugin_Check_CustomNameserver(t *testing.T) {
	mock := &mockResolver{
		LookupHostFunc: func(ctx context.Context, host string, nameserver string) ([]string, error) {
			if nameserver != "8.8.8.8:53" {
				return nil, errors.New("wrong nameserver")
			}
			return []string{"1.1.1.1"}, nil
		},
	}

	plugin := &dnsPlugin{resolver: mock}
	config := regletsdk.Config{
		"hostname":   "example.com",
		"nameserver": "8.8.8.8:53",
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if !evidence.Status {
		t.Error("Expected status true, got false")
	}
}
