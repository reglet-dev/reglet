package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	regletsdk "github.com/whiskeyjimbo/reglet/sdk"
	_ "github.com/whiskeyjimbo/reglet/sdk/net" // Import to enable WASM HTTP transport
)

// httpPlugin implements the sdk.Plugin interface.
type httpPlugin struct {
	// client interface for testing?
	// The SDK intercepts http.DefaultTransport.
	// For testing, we can just use http.Client with a custom Transport (mock).
	// But the plugin code just uses http.NewRequest or http.Get.
	// We can inject a client if we want, or just use http.DefaultClient.
	// Using a field allows injection.
	client *http.Client
}

// Describe returns plugin metadata.
func (p *httpPlugin) Describe(ctx context.Context) (regletsdk.Metadata, error) {
	return regletsdk.Metadata{
		Name:        "http",
		Version:     "1.0.0",
		Description: "HTTP/HTTPS request checking and validation",
		Capabilities: []regletsdk.Capability{
			{
				Kind:    "network",
				Pattern: "outbound:80,443",
			},
		},
	}, nil
}

type HTTPConfig struct {
	URL                  string `json:"url" validate:"required,url" description:"URL to request"`
	Method               string `json:"method" validate:"oneof=GET POST PUT DELETE HEAD OPTIONS PATCH" default:"GET" description:"HTTP method"`
	Body                 string `json:"body,omitempty" description:"Request body"`
	ExpectedStatus       int    `json:"expected_status,omitempty" description:"Expected HTTP status code (optional)"`
	ExpectedBodyContains string `json:"expected_body_contains,omitempty" description:"String that should be present in response body (optional)"`
}

// Schema returns the JSON schema for the plugin's configuration.
func (p *httpPlugin) Schema(ctx context.Context) ([]byte, error) {
	return regletsdk.GenerateSchema(HTTPConfig{})
}

// Check executes the HTTP observation.
func (p *httpPlugin) Check(ctx context.Context, config regletsdk.Config) (regletsdk.Evidence, error) {
	// Set default method
	if _, ok := config["method"]; !ok {
		config["method"] = "GET"
	}

	var cfg HTTPConfig
	if err := regletsdk.ValidateConfig(config, &cfg); err != nil {
		return regletsdk.ConfigError(err), nil
	}

	// Prepare Request
	var bodyReader io.Reader
	if cfg.Body != "" {
		bodyReader = strings.NewReader(cfg.Body)
	}

	req, err := http.NewRequestWithContext(ctx, cfg.Method, cfg.URL, bodyReader)
	if err != nil {
		return regletsdk.ConfigError(fmt.Errorf("failed to create request: %w", err)), nil
	}

	// Use injected client or default
	client := p.client
	if client == nil {
		client = http.DefaultClient
		client.Timeout = 10 * time.Second // Default timeout
	}

	// Execute Request
	resp, err := client.Do(req)
	if err != nil {
		return regletsdk.NetworkError(fmt.Sprintf("HTTP request failed: %v", err), err), nil
	}
	defer resp.Body.Close()

	// Read Body
	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return regletsdk.NetworkError(fmt.Sprintf("failed to read response body: %v", err), err), nil
	}
	respBody := string(respBodyBytes)

	// Collect Result Data
	result := map[string]interface{}{
		"status_code": resp.StatusCode,
		"body_size":   len(respBodyBytes),
		// "headers": resp.Header, // Can be verbose
		"body": respBody, // Include body for evidence? Careful with size.
		// Maybe truncate body?
	}

	// Validate Expectations
	if cfg.ExpectedStatus != 0 {
		if resp.StatusCode != cfg.ExpectedStatus {
			result["expectation_failed"] = true
			result["expectation_error"] = fmt.Sprintf("expected status %d, got %d", cfg.ExpectedStatus, resp.StatusCode)
			// Return success (valid observation) but with expectation failure data?
			// Or failure?
			// Previous implementation returned success response with "expectation_failed" flag.
			return regletsdk.Success(result), nil
		}
	}

	if cfg.ExpectedBodyContains != "" {
		if !strings.Contains(respBody, cfg.ExpectedBodyContains) {
			result["expectation_failed"] = true
			result["expectation_error"] = fmt.Sprintf("expected body to contain '%s'", cfg.ExpectedBodyContains)
			return regletsdk.Success(result), nil
		}
	}

	return regletsdk.Success(result), nil
}
