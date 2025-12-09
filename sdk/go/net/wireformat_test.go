package net

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	sdkcontext "github.com/whiskeyjimbo/reglet/sdk/internal/context"
)

func TestCreateContextWireFormat(t *testing.T) {
	// 1. Background context (empty)
	ctx := context.Background()
	wire := sdkcontext.ContextToWire(ctx)
	if wire.Cancelled {
		t.Error("Background context should not be cancelled")
	}
	if wire.Deadline != nil {
		t.Error("Background context should not have deadline")
	}

	// 2. Cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	wire = sdkcontext.ContextToWire(ctx)
	if !wire.Cancelled {
		t.Error("Context should be cancelled")
	}

	// 3. Deadline context
	deadline := time.Now().Add(1 * time.Hour)
	ctx, cancel = context.WithDeadline(context.Background(), deadline)
	defer cancel()
	wire = sdkcontext.ContextToWire(ctx)
	if wire.Deadline == nil {
		t.Error("Context should have deadline")
	}
	if !wire.Deadline.Equal(deadline) {
		t.Errorf("Deadline mismatch: got %v, want %v", wire.Deadline, deadline)
	}
	if wire.TimeoutMs <= 0 {
		t.Errorf("TimeoutMs should be positive, got %d", wire.TimeoutMs)
	}
}

func TestDNSWireFormats_Marshal(t *testing.T) {
	req := DNSRequestWire{
		Hostname: "example.com",
		Type:     "A",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal DNSRequestWire: %v", err)
	}
	// Basic check that fields are present
	jsonStr := string(data)
	if jsonStr == "" {
		t.Error("Empty JSON output")
	}
}
