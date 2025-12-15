//go:build wasip1

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	regletsdk "github.com/whiskeyjimbo/reglet/sdk"
	regletnet "github.com/whiskeyjimbo/reglet/sdk/net"
)

// httpPlugin implements the sdk.Plugin interface.
type httpPlugin struct {
	// client is an optional HTTP client for testing purposes.
	// If nil, the SDK's default WASM transport is used.
	client *http.Client
}

// Describe returns HTTP plugin metadata.
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
	BodyPreviewLength    int    `json:"body_preview_length,omitempty" default:"200" description:"Number of characters to include from response body (0 = hash only, -1 = full body)"`
}

// Schema returns config schema.
func (p *httpPlugin) Schema(ctx context.Context) ([]byte, error) {
	return regletsdk.GenerateSchema(HTTPConfig{})
}

// Check executes HTTP request.
func (p *httpPlugin) Check(ctx context.Context, config regletsdk.Config) (regletsdk.Evidence, error) {
	// Set default method
	if _, ok := config["method"]; !ok {
		config["method"] = "GET"
	}

	// Set default body_preview_length
	if _, ok := config["body_preview_length"]; !ok {
		config["body_preview_length"] = 200
	}

	var cfg HTTPConfig
	if err := regletsdk.ValidateConfig(config, &cfg); err != nil {
		return regletsdk.Evidence{
			Status: false,
			Error:  regletsdk.ToErrorDetail(&regletsdk.ConfigError{Err: err}),
		}, nil
	}

	// Prepare Request
	var bodyReader io.Reader
	if cfg.Body != "" {
		bodyReader = strings.NewReader(cfg.Body)
	}

	req, err := http.NewRequestWithContext(ctx, cfg.Method, cfg.URL, bodyReader)
	if err != nil {
		return regletsdk.Evidence{Status: false, Error: regletsdk.ToErrorDetail(&regletsdk.ConfigError{Err: fmt.Errorf("failed to create request: %w", err)})}, nil
	}

	// Execute Request using SDK's HTTP helper (which uses WasmTransport)
	// This is more efficient than creating a new client each time
	start := time.Now()
	var resp *http.Response
	if p.client != nil {
		resp, err = p.client.Do(req)
	} else {
		resp, err = regletnet.Do(req)
	}
	duration := time.Since(start).Milliseconds()
	if err != nil {
		return regletsdk.Evidence{
			Status: false,
			Error: regletsdk.ToErrorDetail(
				&regletsdk.NetworkError{
					Operation: "http_request",
					Target:    cfg.URL,
					Err:       err,
				},
			),
		}, nil
	}
	defer resp.Body.Close()

	// Read Body
	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return regletsdk.Evidence{
			Status: false,
			Error: regletsdk.ToErrorDetail(
				&regletsdk.NetworkError{
					Operation: "http_read_body",
					Target:    cfg.URL,
					Err:       err,
				},
			),
		}, nil
	}

	// Calculate body hash for verification
	hash := sha256.Sum256(respBodyBytes)
	bodyHash := hex.EncodeToString(hash[:])

	// Collect Result Data
	result := map[string]interface{}{
		"status_code":      resp.StatusCode,
		"response_time_ms": duration,
		"protocol":         resp.Proto,
		"headers":          resp.Header,
		"body_size":        len(respBodyBytes),
		"body_sha256":      bodyHash,
	}

	// Include body content based on configuration
	if cfg.BodyPreviewLength == -1 {
		// Include full body
		result["body"] = string(respBodyBytes)
	} else if cfg.BodyPreviewLength > 0 {
		// Include preview
		respBody := string(respBodyBytes)
		if len(respBody) > cfg.BodyPreviewLength {
			result["body_preview"] = respBody[:cfg.BodyPreviewLength] + "..."
			result["body_truncated"] = true
		} else {
			result["body"] = respBody
		}
	}
	// If BodyPreviewLength == 0, only include hash and size (no body content)

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
		respBody := string(respBodyBytes)
		if !strings.Contains(respBody, cfg.ExpectedBodyContains) {
			result["expectation_failed"] = true
			result["expectation_error"] = fmt.Sprintf("expected body to contain '%s'", cfg.ExpectedBodyContains)
			return regletsdk.Success(result), nil
		}
	}

	return regletsdk.Success(result), nil
}
