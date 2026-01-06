//go:build wasip1

// Package main provides a complete example Reglet plugin.
//
// This example demonstrates:
// - Plugin interface implementation (Describe, Schema, Check)
// - Safe config extraction using SDK helpers
// - HTTP requests with context
// - Proper error handling
// - Evidence generation
//
// Build:
//
//	GOOS=wasip1 GOARCH=wasm go build -o basic-plugin.wasm main.go
//
// Run with Reglet:
//
//	reglet check --profile test-profile.yaml
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	sdk "github.com/whiskeyjimbo/reglet/sdk"
	_ "github.com/whiskeyjimbo/reglet/sdk/log" // Initialize WASM logging
	sdknet "github.com/whiskeyjimbo/reglet/sdk/net"
)

// BasicPlugin is a simple example plugin that checks HTTP endpoint availability.
type BasicPlugin struct{}

func main() {
	sdk.Register(&BasicPlugin{})
}

// Describe returns plugin metadata.
func (p *BasicPlugin) Describe(ctx context.Context) (sdk.Metadata, error) {
	return sdk.Metadata{
		Name:        "basic-http-check",
		Version:     "1.0.0",
		Description: "Checks HTTP endpoint availability and returns status code",
		Capabilities: []sdk.Capability{
			{Kind: "network:outbound", Pattern: "*:443"},
			{Kind: "network:outbound", Pattern: "*:80"},
		},
	}, nil
}

// Schema returns JSON schema for plugin configuration.
func (p *BasicPlugin) Schema(ctx context.Context) ([]byte, error) {
	type Config struct {
		URL           string `json:"url" description:"HTTP(S) URL to check"`
		ExpectedCode  int    `json:"expected_code,omitempty" description:"Expected HTTP status code" default:"200"`
		CheckContains string `json:"check_contains,omitempty" description:"String that response body should contain"`
	}
	return sdk.GenerateSchema(Config{})
}

// Check performs the HTTP endpoint check.
func (p *BasicPlugin) Check(ctx context.Context, config sdk.Config) (sdk.Evidence, error) {
	// Extract required config using SDK helpers (safe - no panics)
	url, err := sdk.MustGetString(config, "url")
	if err != nil {
		return sdk.Failure("config", err.Error()), nil
	}

	// Extract optional config with defaults
	expectedCode := sdk.GetIntDefault(config, "expected_code", 200)
	checkContains := sdk.GetStringDefault(config, "check_contains", "")

	slog.InfoContext(ctx, "Starting HTTP check",
		"url", url,
		"expected_code", expectedCode,
	)

	// Make HTTP request
	resp, err := sdknet.Get(ctx, url)
	if err != nil {
		slog.ErrorContext(ctx, "HTTP request failed", "error", err)
		return sdk.Evidence{
			Status: false,
			Error:  sdk.ToErrorDetail(err),
			Data: map[string]interface{}{
				"url":   url,
				"error": err.Error(),
			},
		}, nil
	}
	defer resp.Body.Close()

	// Read body if we need to check contents
	var bodyContains bool
	if checkContains != "" {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.WarnContext(ctx, "Failed to read response body", "error", err)
		} else {
			bodyContains = contains(string(body), checkContains)
		}
	}

	// Evaluate pass/fail
	codeMatch := resp.StatusCode == expectedCode
	contentMatch := checkContains == "" || bodyContains

	passed := codeMatch && contentMatch

	evidence := sdk.Evidence{
		Status: passed,
		Data: map[string]interface{}{
			"url":               url,
			"status_code":       resp.StatusCode,
			"expected_code":     expectedCode,
			"code_matches":      codeMatch,
			"content_matches":   contentMatch,
			"content_checked":   checkContains != "",
			"content_substring": checkContains,
		},
	}

	if !passed {
		evidence.Error = sdk.ToErrorDetail(fmt.Errorf(
			"check failed: status=%d (expected %d), content_match=%v",
			resp.StatusCode, expectedCode, contentMatch,
		))
	}

	slog.InfoContext(ctx, "Check completed",
		"passed", passed,
		"status_code", resp.StatusCode,
	)

	return evidence, nil
}

// contains checks if substr is in s (simple implementation).
func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
