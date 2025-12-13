// Package main provides a file plugin for Reglet.
// This is compiled to WASM and loaded by the Reglet runtime.
//go:build wasip1

package main

import (
	"log/slog"

	regletsdk "github.com/whiskeyjimbo/reglet/sdk"
)

func init() {
	slog.Info("File plugin init() started")
	regletsdk.Register(&filePlugin{})
	slog.Info("File plugin init() registered")
}

// main function for the WASM plugin.
func main() {}
