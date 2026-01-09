// Package main provides an HTTP plugin for Reglet.
// This is compiled to WASM and loaded by the Reglet runtime.
//go:build wasip1

package main

import (
	"log/slog"

	regletsdk "github.com/reglet-dev/reglet/sdk"
)

func init() {
	slog.Info("HTTP plugin init() started")
	regletsdk.Register(&httpPlugin{})
	slog.Info("HTTP plugin init() registered")
}

// main function for the WASM plugin.
func main() {}
