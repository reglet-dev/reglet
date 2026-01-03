//go:build wasip1

package main

import (
	"log/slog"

	regletsdk "github.com/whiskeyjimbo/reglet/sdk"
)

func init() {
	slog.Info("AWS plugin init() started")
	regletsdk.Register(&awsPlugin{})
	slog.Info("AWS plugin init() registered")
}

// main function for the WASM plugin.
func main() {}
