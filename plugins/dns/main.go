// Package main provides a dns plugin for Reglet.
// This is compiled to WASM and loaded by the Reglet runtime.
//go:build wasip1

package main

import (
	"context"
	"log/slog"

	regletsdk "github.com/whiskeyjimbo/reglet/sdk"     // Import the new SDK
	regletnet "github.com/whiskeyjimbo/reglet/sdk/net" // Import SDK net package
)

// wasmResolver adapts the SDK's static functions to the dnsResolver interface.
type wasmResolver struct{}

func (w *wasmResolver) LookupHost(ctx context.Context, host string, nameserver string) ([]string, error) {
	r := &regletnet.WasmResolver{Nameserver: nameserver}
	return r.LookupHost(ctx, host)
}

func (w *wasmResolver) LookupCNAME(ctx context.Context, host string, nameserver string) (string, error) {
	r := &regletnet.WasmResolver{Nameserver: nameserver}
	return r.LookupCNAME(ctx, host)
}

func (w *wasmResolver) LookupMX(ctx context.Context, host string, nameserver string) ([]string, error) {
	r := &regletnet.WasmResolver{Nameserver: nameserver}
	return r.LookupMX(ctx, host)
}

func (w *wasmResolver) LookupTXT(ctx context.Context, host string, nameserver string) ([]string, error) {
	r := &regletnet.WasmResolver{Nameserver: nameserver}
	return r.LookupTXT(ctx, host)
}

func (w *wasmResolver) LookupNS(ctx context.Context, host string, nameserver string) ([]string, error) {
	r := &regletnet.WasmResolver{Nameserver: nameserver}
	return r.LookupNS(ctx, host)
}

func init() {
	slog.Info("DNS plugin init() started")
	// Inject the real WASM-compatible resolver
	regletsdk.Register(&dnsPlugin{})
	slog.Info("DNS plugin init() registered")
}

// main function for the WASM plugin.
func main() {}
