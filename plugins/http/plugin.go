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

	regletsdk "github.com/reglet-dev/reglet/sdk"
	regletnet "github.com/reglet-dev/reglet/sdk/net"
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
	cfg, err := parseHTTPConfig(config)
	if err != nil {
		return regletsdk.Evidence{Status: false, Error: regletsdk.ToErrorDetail(err)}, nil
	}

	resp, respBody, duration, err := p.executeRequest(ctx, cfg)
	if err != nil {
		return regletsdk.Evidence{Status: false, Error: regletsdk.ToErrorDetail(err)}, nil
	}
	defer resp.Body.Close()

	result := buildHTTPResult(resp, respBody, duration, cfg)

	if err := validateExpectations(cfg, resp, respBody, result); err != nil {
		return regletsdk.Success(result), nil
	}

	return regletsdk.Success(result), nil
}

// parseHTTPConfig validates and parses the config with defaults.
func parseHTTPConfig(config regletsdk.Config) (*HTTPConfig, error) {
	// Set defaults
	if _, ok := config["method"]; !ok {
		config["method"] = "GET"
	}
	if _, ok := config["body_preview_length"]; !ok {
		config["body_preview_length"] = 200
	}

	var cfg HTTPConfig
	if err := regletsdk.ValidateConfig(config, &cfg); err != nil {
		return nil, &regletsdk.ConfigError{Err: err}
	}
	return &cfg, nil
}

// executeRequest performs the HTTP request.
func (p *httpPlugin) executeRequest(ctx context.Context, cfg *HTTPConfig) (*http.Response, []byte, int64, error) {
	var bodyReader io.Reader
	if cfg.Body != "" {
		bodyReader = strings.NewReader(cfg.Body)
	}

	req, err := http.NewRequestWithContext(ctx, cfg.Method, cfg.URL, bodyReader)
	if err != nil {
		return nil, nil, 0, &regletsdk.ConfigError{Err: fmt.Errorf("failed to create request: %w", err)}
	}

	start := time.Now()
	var resp *http.Response
	if p.client != nil {
		resp, err = p.client.Do(req)
	} else {
		resp, err = regletnet.Do(req)
	}
	duration := time.Since(start).Milliseconds()

	if err != nil {
		return nil, nil, 0, &regletsdk.NetworkError{Operation: "http_request", Target: cfg.URL, Err: err}
	}

	respBodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, 0, &regletsdk.NetworkError{Operation: "http_read_body", Target: cfg.URL, Err: err}
	}

	return resp, respBodyBytes, duration, nil
}

// buildHTTPResult constructs the result map from the response.
func buildHTTPResult(resp *http.Response, respBody []byte, duration int64, cfg *HTTPConfig) map[string]interface{} {
	hash := sha256.Sum256(respBody)
	bodyHash := hex.EncodeToString(hash[:])

	result := map[string]interface{}{
		"status_code":      resp.StatusCode,
		"response_time_ms": duration,
		"protocol":         resp.Proto,
		"headers":          resp.Header,
		"body_size":        len(respBody),
		"body_sha256":      bodyHash,
	}

	addBodyContent(result, respBody, cfg.BodyPreviewLength)
	return result
}

// addBodyContent adds body content to result based on configuration.
func addBodyContent(result map[string]interface{}, respBody []byte, previewLength int) {
	switch {
	case previewLength == -1:
		result["body"] = string(respBody)
	case previewLength > 0:
		respBodyStr := string(respBody)
		if len(respBodyStr) > previewLength {
			result["body_preview"] = respBodyStr[:previewLength] + "..."
			result["body_truncated"] = true
		} else {
			result["body"] = respBodyStr
		}
	}
	// If previewLength == 0, only include hash and size (no body content)
}

// validateExpectations checks if response matches expected values.
func validateExpectations(cfg *HTTPConfig, resp *http.Response, respBody []byte, result map[string]interface{}) error {
	if cfg.ExpectedStatus != 0 && resp.StatusCode != cfg.ExpectedStatus {
		result["expectation_failed"] = true
		result["expectation_error"] = fmt.Sprintf("expected status %d, got %d", cfg.ExpectedStatus, resp.StatusCode)
		return fmt.Errorf("status mismatch")
	}

	if cfg.ExpectedBodyContains != "" && !strings.Contains(string(respBody), cfg.ExpectedBodyContains) {
		result["expectation_failed"] = true
		result["expectation_error"] = fmt.Sprintf("expected body to contain '%s'", cfg.ExpectedBodyContains)
		return fmt.Errorf("body mismatch")
	}

	return nil
}
