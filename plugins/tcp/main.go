// Package main provides a TCP plugin for Reglet.
// This is compiled to WASM and loaded by the Reglet runtime.
//go:build wasip1

package main

import (
	"log/slog"

	regletsdk "github.com/whiskeyjimbo/reglet/sdk"
	regletnet "github.com/whiskeyjimbo/reglet/sdk/net"
)

func init() {
	slog.Info("TCP plugin init() started")
	regletsdk.Register(&tcpPlugin{DialTCP: regletnet.DialTCP})
	slog.Info("TCP plugin init() registered")
}

// main function for the WASM plugin.
func main() {}