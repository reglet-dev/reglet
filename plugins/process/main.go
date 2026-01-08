// Package main provides a process plugin for Reglet.
// This is compiled to WASM and loaded by the Reglet runtime.
//go:build wasip1

package main

import (
	"log/slog"

	regletsdk "github.com/whiskeyjimbo/reglet/sdk"
)

func init() {
	slog.Info("Process plugin init() started")
	regletsdk.Register(&processPlugin{})
	slog.Info("Process plugin init() registered")
}

// main is the entry point for the WASM module.
// It is required for WASM compilation but uses the SDK for logic.
func main() {}
