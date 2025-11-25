// Package main provides the Reglet CLI entry point.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// TODO: Implement CLI using cobra or similar
	fmt.Println("Reglet - Compliance and Infrastructure Validation")
	fmt.Println("Version: dev")
	return nil
}
